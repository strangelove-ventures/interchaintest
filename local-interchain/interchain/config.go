package interchain

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	types "github.com/strangelove-ventures/localinterchain/interchain/types"
	"github.com/strangelove-ventures/localinterchain/interchain/util"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

func loadConfig(config *types.Config, filepath string) (*types.Config, error) {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func LoadConfig(installDir, chainCfgFile string) (*types.Config, error) {
	var config *types.Config

	configFile := "base.json"
	if chainCfgFile != "" {
		configFile = chainCfgFile
	}

	// Chains Folder
	chainsDir := filepath.Join(installDir, "chains")
	cfgFilePath := filepath.Join(chainsDir, configFile)

	config, err := loadConfig(config, cfgFilePath)
	if err != nil {
		return nil, err
	}

	log.Println("Using directory:", installDir)
	log.Println("Using chain config:", cfgFilePath)

	chains := config.Chains

	for i := range chains {
		chain := chains[i]
		chain.SetChainDefaults()
		util.ReplaceStringValues(&chain, "%DENOM%", chain.Denom)
		util.ReplaceStringValues(&chain, "%BIN%", chain.Binary)
		util.ReplaceStringValues(&chain, "%CHAIN_ID%", chain.ChainID)

		chains[i] = chain

		if config.Chains[i].Debugging {
			fmt.Printf("Loaded %v\n", config)
		}
	}

	return config, nil
}

func FasterBlockTimesBuilder(blockTime string) testutil.Toml {
	if _, err := time.ParseDuration(blockTime); err != nil {
		panic(err)
	}

	tomlCfg := testutil.Toml{
		"consensus": testutil.Toml{
			"timeout_commit":  blockTime,
			"timeout_propose": blockTime,
		},
	}

	return testutil.Toml{"config/config.toml": tomlCfg}
}

func CreateChainConfigs(cfg types.Chain) (ibc.ChainConfig, *interchaintest.ChainSpec) {

	hostPorts := make(map[int]int, len(cfg.HostPortOverride))
	for k, v := range cfg.HostPortOverride {
		internalPort, err := strconv.Atoi(k)
		if err != nil {
			panic(err)
		}
		externalPort, err := strconv.Atoi(v)
		if err != nil {
			panic(err)
		}

		hostPorts[internalPort] = externalPort
	}

	chainCfg := ibc.ChainConfig{
		Type:             cfg.ChainType,
		Name:             cfg.Name,
		ChainID:          cfg.ChainID,
		Bin:              cfg.Binary,
		Bech32Prefix:     cfg.Bech32Prefix,
		Denom:            cfg.Denom,
		CoinType:         fmt.Sprintf("%d", cfg.CoinType),
		GasPrices:        cfg.GasPrices,
		GasAdjustment:    cfg.GasAdjustment,
		TrustingPeriod:   cfg.TrustingPeriod,
		HostPortOverride: hostPorts,

		// TODO: Allow host mount in the future
		NoHostMount:         false,
		ModifyGenesis:       cosmos.ModifyGenesis(cfg.Genesis.Modify),
		ConfigFileOverrides: FasterBlockTimesBuilder(cfg.BlockTime),
		EncodingConfig:      nil,
	}

	if cfg.DockerImage.Version == "" {
		panic("DockerImage.Version is required in your config")
	}

	if cfg.DockerImage.Repository != "" {
		chainCfg.Images = []ibc.DockerImage{
			{
				Repository: cfg.DockerImage.Repository,
				Version:    cfg.DockerImage.Version,
				UidGid:     cfg.DockerImage.UidGid,
			},
		}
	}

	chainSpecs := &interchaintest.ChainSpec{
		Name:          cfg.Name,
		Version:       cfg.DockerImage.Version,
		ChainName:     cfg.ChainID,
		ChainConfig:   chainCfg,
		NumValidators: &cfg.NumberVals,
		NumFullNodes:  &cfg.NumberNode,
	}

	return chainCfg, chainSpecs
}
