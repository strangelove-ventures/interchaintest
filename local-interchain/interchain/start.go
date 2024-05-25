package interchain

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"path"
	"strings"

	"github.com/strangelove-ventures/interchaintest/localic/interchain/router"
	"github.com/strangelove-ventures/interchaintest/localic/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
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

	// TODO: cleanup on ctrl + c
	// Properly cleanup at the start, during build, and after the REST API is created
	// The following only works at the start and after the REST API is created.
	//
	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt)
	// go func() {
	// 	for sig := range c {
	// 		log.Printf("Closing from signal: %s\n", sig)
	// 		handlers.KillAll(ctx, ic, vals, relayer, eRep)
	// 	}
	// }()

	// Logger for ICTest functions only.
	logger, err := InitLogger()
	if err != nil {
		panic(err)
	}

	config := ac.Cfg

	config.Relayer = ac.Relayer

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
		log.Fatal("VerifyIBCPaths", err)
	}

	// Create chain factory for all the chains
	cf := interchaintest.NewBuiltinChainFactory(logger, chainSpecs)

	testName := GetTestName(chainCfgFile)

	chains, err := cf.Chains(testName)
	if err != nil {
		log.Fatal("cf.Chains", err)
	}

	for _, chain := range chains {
		ic = ic.AddChain(chain)
	}
	ic.AdditionalGenesisWallets = SetupGenesisWallets(config, chains)

	fakeT := FakeTesting{
		FakeName: testName,
	}

	// Base setup
	rep := testreporter.NewNopReporter()
	eRep = rep.RelayerExecReporter(&fakeT)

	client, network := interchaintest.DockerSetup(fakeT)

	// setup a relayer if we have IBC paths to use.
	if len(ibcpaths) > 0 || len(icsPair) > 0 {
		rlyCfg := config.Relayer

		relayerType, relayerName := ibc.CosmosRly, "relay"
		rf := interchaintest.NewBuiltinRelayerFactory(
			relayerType,
			logger,
			interchaintestrelayer.CustomDockerImage(
				rlyCfg.DockerImage.Repository,
				rlyCfg.DockerImage.Version,
				rlyCfg.DockerImage.UidGid,
			),
			interchaintestrelayer.StartupFlags(rlyCfg.StartupFlags...),
		)

		// This also just needs the name.
		relayer = rf.Build(fakeT, client, network)
		ic = ic.AddRelayer(relayer, relayerName)

		// Add links between chains
		LinkIBCPaths(ibcpaths, chains, ic, relayer)
	}

	// Add Interchain Security chain pairs together
	icsProviderPaths := make(map[string]ibc.Chain)
	if len(icsPair) > 0 {
		for provider, consumers := range icsPair {
			var p, c ibc.Chain

			// a provider can have multiple consumers
			for _, consumer := range consumers {
				for _, chain := range chains {
					if chain.Config().ChainID == provider {
						p = chain
					}
					if chain.Config().ChainID == consumer {
						c = chain
					}
				}
			}

			pathName := fmt.Sprintf("%s-%s", p.Config().ChainID, c.Config().ChainID)

			logger.Info("Adding ICS pair", zap.String("provider", p.Config().ChainID), zap.String("consumer", c.Config().ChainID), zap.String("path", pathName))

			icsProviderPaths[pathName] = p

			ic = ic.AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
				Provider: p,
				Consumer: c,
				Relayer:  relayer,
				Path:     pathName,
			})
		}
	}

	// Build all chains & begin.
	err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         testName,
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
	})
	if err != nil {
		logger.Fatal("ic.Build", zap.Error(err))
	}

	if relayer != nil && len(ibcpaths) > 0 {
		paths := make([]string, 0, len(ibcpaths))
		for k := range ibcpaths {
			paths = append(paths, k)
		}

		if err := relayer.StartRelayer(ctx, eRep, paths...); err != nil {
			log.Fatal("relayer.StartRelayer", err)
		}

		defer func() {
			if err := relayer.StopRelayer(ctx, eRep); err != nil {
				log.Fatal("relayer.StopRelayer", err)
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
					log.Fatal("FinishICSProviderSetup", err)
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

		r := router.NewRouter(ctx, ic, config, cosmosChains, vals, relayer, ac.AuthKey, eRep, installDir)

		config.Server = types.RestServer{
			Host: ac.Address,
			Port: fmt.Sprintf("%d", ac.Port),
		}

		server := fmt.Sprintf("%s:%s", config.Server.Host, config.Server.Port)
		if err := http.ListenAndServe(server, r); err != nil {
			log.Default().Println(err)
		}
	}()

	AddGenesisKeysToKeyring(ctx, config, chains)

	// run commands for each server after startup. Iterate chain configs
	PostStartupCommands(ctx, config, chains)

	connections := GetChannelConnections(ctx, ibcpaths, chains, ic, relayer, eRep)

	// Save to logs.json file for runtime chain information.
	DumpChainsInfoToLogs(installDir, config, chains, connections)

	log.Println("\nLocal-IC API is running on ", fmt.Sprintf("http://%s:%s", config.Server.Host, config.Server.Port))

	if err = testutil.WaitForBlocks(ctx, math.MaxInt, chains[0]); err != nil {
		log.Fatal("WaitForBlocks StartChain: ", err)
	}
}

func GetTestName(chainCfgFile string) string {
	name := chainCfgFile
	fExt := path.Ext(name)
	if fExt != "" {
		name = strings.ReplaceAll(chainCfgFile, fExt, "")
	}

	return name + "ic"
}
