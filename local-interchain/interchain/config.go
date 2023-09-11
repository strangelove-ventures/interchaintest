package interchain

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	types "github.com/strangelove-ventures/localinterchain/interchain/types"
	"github.com/strangelove-ventures/localinterchain/interchain/util"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
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

	// configs Folder
	configsDir := filepath.Join(installDir, "configs")
	relayerFilePath := filepath.Join(configsDir, "relayer.json")
	serverFilePath := filepath.Join(configsDir, "server.json")

	config, err := loadConfig(config, cfgFilePath)
	if err != nil {
		return nil, err
	}
	config, _ = loadConfig(config, relayerFilePath)
	config, _ = loadConfig(config, serverFilePath)

	log.Println("Using directory:", installDir)
	log.Println("Using chain config:", cfgFilePath)

	chains := config.Chains
	relayer := config.Relayer

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

	config.Relayer = relayer.SetRelayerDefaults()

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
	chainCfg := ibc.ChainConfig{
		Type:                cfg.ChainType,
		Name:                cfg.Name,
		ChainID:             cfg.ChainID,
		Bin:                 cfg.Binary,
		Bech32Prefix:        cfg.Bech32Prefix,
		Denom:               cfg.Denom,
		CoinType:            fmt.Sprintf("%d", cfg.CoinType),
		GasPrices:           cfg.GasPrices,
		GasAdjustment:       cfg.GasAdjustment,
		TrustingPeriod:      cfg.TrustingPeriod,
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
