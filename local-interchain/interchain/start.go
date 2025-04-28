package interchain

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/router"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
)

func StartChain(installDir, chainCfgFile string, ac *types.AppStartConfig) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var relayer ibc.Relayer
	var eRep *testreporter.RelayerExecReporter

	vals := make(map[string][]*cosmos.ChainNode)
	ic := interchaintest.NewInterchain()
	defer ic.Close()

	// Cleanup servers on ctrl+c signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-c:
			fmt.Println("\nReceived signal to stop local-ic...")
			killContainer(ctx)
		case <-ctx.Done():
			killContainer(ctx)
		}
	}()

	// very unique file to ensure if multiple start at the same time.
	logFile, err := interchaintest.CreateLogFile(fmt.Sprintf("%d-%s.json", time.Now().Unix(), uuid.New()))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Println("Error closing log file: ", err)
		}

		if err := os.Remove(logFile.Name()); err != nil {
			fmt.Println("Error deleting log file: ", err)
		}
	}()

	// Logger for ICTest functions only.
	logger, err := InitLogger(logFile)
	if err != nil {
		panic(err)
	}
	logger.Debug("Log file created", zap.String("file", logFile.Name()))

	config := ac.Cfg

	WriteRunningChains(installDir, []byte("{}"))

	// ibc-path-name -> index of []cosmos.CosmosChain
	ibcpaths := make(map[string][]int)
	// providerChainId -> []consumerChainIds
	icsPair := make(map[string][]string)

	chainSpecs := []*interchaintest.ChainSpec{}

	for idx, cfg := range config.Chains {
		_, chainSpec := CreateChainConfigs(cfg)
		chainSpecs = append(chainSpecs, chainSpec)

		if len(cfg.IBCPaths) > 0 {
			for _, path := range cfg.IBCPaths {
				ibcpaths[path] = append(ibcpaths[path], idx)
			}
		}

		if cfg.ICSConsumerLink != "" {
			icsPair[cfg.ICSConsumerLink] = append(icsPair[cfg.ICSConsumerLink], cfg.ChainID)
		}
	}

	if err := VerifyIBCPaths(ibcpaths); err != nil {
		logger.Fatal("VerifyIBCPaths", zap.Error(err))
	}

	// Create chain factory for all the chains
	cf := interchaintest.NewBuiltinChainFactory(logger, chainSpecs)

	testName := GetTestName(chainCfgFile)

	chains, err := cf.Chains(testName)
	if err != nil {
		logger.Fatal("ChainFactory chains", zap.Error(err))
	}

	for _, chain := range chains {
		ic = ic.AddChain(chain)
	}
	ic.AdditionalGenesisWallets = SetupGenesisWallets(config, chains)

	fakeT := FakeTesting{
		FakeName: testName,
	}

	// Base setup

	rep := testreporter.NewReporter(logFile)
	eRep = rep.RelayerExecReporter(&fakeT)

	client, network := interchaintest.DockerSetup(fakeT)

	// setup a relayer if we have IBC paths to use.
	if len(ibcpaths) > 0 || len(icsPair) > 0 {
		rlyCfg := config.Relayer
		rf := interchaintest.NewBuiltinRelayerFactory(
			rlyCfg.Type,
			logger,
			interchaintestrelayer.CustomDockerImage(
				rlyCfg.DockerImage.Repository,
				rlyCfg.DockerImage.Version,
				rlyCfg.DockerImage.UIDGID,
			),
			interchaintestrelayer.StartupFlags(*rlyCfg.StartupFlags...),
		)

		// This also just needs the name.
		relayer = rf.Build(fakeT, client, network)
		ic = ic.AddRelayer(relayer, "relay")

		// Add links between chains
		LinkIBCPaths(ibcpaths, chains, ic, relayer)
	}

	// Add Interchain Security chain pairs together
	icsProviderPaths := make(map[string]ibc.Chain)
	if len(icsPair) > 0 {
		for provider, consumers := range icsPair {
			var p ibc.Chain

			allConsumers := []ibc.Chain{}
			for _, consumer := range consumers {
				for _, chain := range chains {
					if chain.Config().ChainID == provider {
						p = chain
					}
					if chain.Config().ChainID == consumer {
						allConsumers = append(allConsumers, chain)
					}
				}
			}

			if p == nil {
				logger.Fatal("provider not found in chains on Start", zap.String("provider", provider))
			}

			for _, c := range allConsumers {
				pathName := fmt.Sprintf("%s-%s", p.Config().ChainID, c.Config().ChainID)

				logger.Info("Adding ICS pair", zap.String("provider", p.Config().ChainID), zap.String("consumer", c.Config().ChainID), zap.String("path", pathName))

				if _, ok := icsProviderPaths[pathName]; ok {
					logger.Fatal("pathName already exists in icsProviderPaths. Update the consumers ChainID to be unique", zap.String("pathName", pathName))
				}

				icsProviderPaths[pathName] = p

				ic = ic.AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
					Provider: p,
					Consumer: c,
					Relayer:  relayer,
					Path:     pathName,
				})
			}
		}
	}

	// Build all chains & begin.
	err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         testName,
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	})
	if err != nil {
		logger.Fatal("Interchain Build", zap.Error(err))
	}

	if relayer != nil && len(ibcpaths) > 0 {
		paths := make([]string, 0, len(ibcpaths))
		for k := range ibcpaths {
			paths = append(paths, k)
		}

		if err := relayer.StartRelayer(ctx, eRep, paths...); err != nil {
			logger.Fatal("Relayer StartRelayer", zap.Error(err))
		}

		defer func() {
			if err := relayer.StopRelayer(ctx, eRep); err != nil {
				logger.Error("Relayer StopRelayer", zap.Error(err))
			}
		}()
	}

	for _, chain := range chains {
		if cosmosChain, ok := chain.(*cosmos.CosmosChain); ok {
			chainID := cosmosChain.Config().ChainID
			vals[chainID] = cosmosChain.Validators
		}
	}

	// ICS provider setup
	if len(icsProviderPaths) > 0 {
		logger.Info("ICS provider setup", zap.Any("icsProviderPaths", icsProviderPaths))

		for ibcPath, chain := range icsProviderPaths {
			if provider, ok := chain.(*cosmos.CosmosChain); ok {
				if err := provider.FinishICSProviderSetup(ctx, relayer, eRep, ibcPath); err != nil {
					logger.Error("FinishICSProviderSetup", zap.Error(err))
				}
			}
		}
	}

	// Starts a non blocking REST server to take action on the chain.
	go func() {
		cosmosChains := map[string]*cosmos.CosmosChain{}
		for _, chain := range chains {
			if cosmosChain, ok := chain.(*cosmos.CosmosChain); ok {
				cosmosChains[cosmosChain.Config().ChainID] = cosmosChain
			}
		}

		r := router.NewRouter(ctx, ic, &router.RouterConfig{
			Logger:              logger,
			RelayerExecReporter: eRep,
			Config:              config,
			CosmosChains:        cosmosChains,
			DockerClient:        client,
			Vals:                vals,
			Relayer:             relayer,
			AuthKey:             ac.AuthKey,
			InstallDir:          installDir,
			LogFile:             logFile.Name(),
			TestName:            testName,
		})

		config.Server = types.RestServer{
			Host: ac.Address,
			Port: fmt.Sprintf("%d", ac.Port),
		}

		if config.Server.Host == "" {
			config.Server.Host = "127.0.0.1"
		}
		if config.Server.Port == "" {
			config.Server.Port = "8080"
		}

		serverAddr := fmt.Sprintf("%s:%s", config.Server.Host, config.Server.Port)

		// Where ORIGIN_ALLOWED is like `scheme://dns[:port]`, or `*` (insecure)
		corsHandler := handlers.CORS(
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "Accept"}),
			handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"}),
			handlers.AllowCredentials(),
			handlers.ExposedHeaders([]string{"*"}),
		)

		if err := http.ListenAndServe(serverAddr, corsHandler(r)); err != nil {
			logger.Error("HTTP ListenAndServe", zap.Error(err))
		}
	}()

	AddGenesisKeysToKeyring(ctx, config, chains)

	// run commands for each server after startup. Iterate chain configs
	PostStartupCommands(ctx, config, chains)

	connections := GetChannelConnections(ctx, ibcpaths, chains, ic, relayer, eRep)

	// Save to logs.json file for runtime chain information.
	DumpChainsInfoToLogs(installDir, config, chains, connections)

	logger.Info("Local-IC API is running", zap.String("url", fmt.Sprintf("http://%s:%s", config.Server.Host, config.Server.Port)))

	if err = testutil.WaitForBlocks(ctx, math.MaxInt, chains[0]); err != nil {
		// when the network is stopped / killed (ctrl + c), ignore error
		if !strings.Contains(err.Error(), "post failed:") {
			fmt.Println("WaitForBlocks StartChain: ", err)
		}
	}

	wg.Wait()
}

func GetTestName(chainCfgFile string) string {
	name := chainCfgFile
	fExt := path.Ext(name)
	if fExt != "" {
		name = strings.ReplaceAll(chainCfgFile, fExt, "")
	}

	return name + "ic"
}

func killContainer(ctx context.Context) {
	removed := dockerutil.KillAllInterchaintestContainers(ctx)
	for _, r := range removed {
		fmt.Println("  - ", r)
	}
	os.Exit(1)
}
