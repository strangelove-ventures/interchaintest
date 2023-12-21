package interchain

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// ConfigurationOverrides creates a map of config file overrides for filenames, their keys, and values.
func ConfigurationOverrides(cfg types.Chain) testutil.Toml {
	var toml testutil.Toml

	switch cfg.ChainType {
	case "cosmos":
		toml = cosmosConfigOverride(cfg)
		fmt.Println("cosmos toml", toml)
	case "ethereum":
		toml = ethereumConfigOverride(cfg)
	default:
		toml = make(testutil.Toml, 0)
	}

	for _, o := range cfg.ConfigFileOverrides {
		for k, v := range o.Paths {
			// create file key if it does not exist
			if _, ok := toml[o.File]; !ok {
				toml[o.File] = testutil.Toml{}
			}

			// if there is no path, save the KV directly without the header
			if !strings.Contains(k, ".") {
				toml[o.File].(testutil.Toml)[k] = v
				continue
			}

			// separate the path and key
			path, key := strings.Split(k, ".")[0], strings.Split(k, ".")[1]
			fmt.Println("file", o.File, "path", path, "key", key, "v", v)

			// create path key if it does not exist
			if _, ok := toml[o.File].(testutil.Toml)[path]; !ok {
				toml[o.File].(testutil.Toml)[path] = testutil.Toml{}
			}

			// save the new KV pair
			toml[o.File].(testutil.Toml)[path].(testutil.Toml)[key] = v
		}
	}

	return toml
}

func cosmosConfigOverride(cfg types.Chain) testutil.Toml {
	blockTime := cfg.BlockTime
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

// TODO: use ConfigurationOverrides instead?
func ethereumConfigOverride(cfg types.Chain) testutil.Toml {
	anvilConfigFileOverrides := make(map[string]any)
	anvilConfigFileOverrides["--load-state"] = cfg.EVMLoadStatePath
	return anvilConfigFileOverrides
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
		ConfigFileOverrides: ConfigurationOverrides(cfg),
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
