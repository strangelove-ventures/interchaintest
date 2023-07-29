package interchain

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/strangelove-ventures/localinterchain/interchain/router"
	"go.uber.org/zap"
)

func StartChain(installDir, chainCfgFile string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var relayer ibc.Relayer
	var eRep *testreporter.RelayerExecReporter

	vals := make(map[string]*cosmos.ChainNode)
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

	config, err := LoadConfig(installDir, chainCfgFile)
	if err != nil {
		// try again with .json, then if it still fails - panic
		config, err = LoadConfig(installDir, chainCfgFile+".json")
		if err != nil {
			panic(err)
		}
	}

	WriteRunningChains(installDir, []byte("{}"))

	// ibc-path-name -> index of []cosmos.CosmosChain
	ibcpaths := make(map[string][]int)
	chainSpecs := []*interchaintest.ChainSpec{}

	for idx, cfg := range config.Chains {
		_, chainSpec := CreateChainConfigs(cfg)
		chainSpecs = append(chainSpecs, chainSpec)

		if len(cfg.IBCPaths) > 0 {
			for _, path := range cfg.IBCPaths {
				ibcpaths[path] = append(ibcpaths[path], idx)
			}
		}
	}

	if err := VerifyIBCPaths(ibcpaths); err != nil {
		log.Fatal("VerifyIBCPaths", err)
	}

	// Create chain factory for all the chains
	cf := interchaintest.NewBuiltinChainFactory(logger, chainSpecs)

	// Get chains from the chain factory
	name := strings.ReplaceAll(chainCfgFile, ".json", "") + "ic"
	chains, err := cf.Chains(name)
	if err != nil {
		log.Fatal("cf.Chains", err)
	}

	for _, chain := range chains {
		ic = ic.AddChain(chain)
	}
	ic.AdditionalGenesisWallets = SetupGenesisWallets(config, chains)

	fakeT := FakeTesting{
		FakeName: name,
	}

	// Base setup
	rep := testreporter.NewNopReporter()
	eRep = rep.RelayerExecReporter(&fakeT)

	client, network := interchaintest.DockerSetup(fakeT)

	// setup a relayer if we have IBC paths to use.
	if len(ibcpaths) > 0 {
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

	// Build all chains & begin.
	err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         name,
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

		relayer.StartRelayer(ctx, eRep, paths...)
		defer func() {
			relayer.StopRelayer(ctx, eRep)
		}()
	}

	for _, chain := range chains {
		if cosmosChain, ok := chain.(*cosmos.CosmosChain); ok {
			chainID := cosmosChain.Config().ChainID
			vals[chainID] = cosmosChain.Validators[0]
		}
	}

	// Starts a non blocking REST server to take action on the chain.
	go func() {
		r := router.NewRouter(ctx, ic, config, vals, relayer, eRep, installDir)

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
