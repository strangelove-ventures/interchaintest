package interchain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	types "github.com/strangelove-ventures/localinterchain/interchain/types"
	"github.com/strangelove-ventures/localinterchain/interchain/util"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

func LoadConfig(installDir, chainCfgFile string) (*types.Config, error) {
	var config types.Config

	configFile := "base.json"
	if chainCfgFile != "" {
		configFile = chainCfgFile
	}

	// Chains Folder
	chainsDir := filepath.Join(installDir, "chains")
	cfgFilePath := filepath.Join(chainsDir, configFile)

	bytes, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	log.Println("Using directory:", installDir)
	log.Println("Using chain config:", cfgFilePath)

	return setConfigDefaults(&config), nil
}

func LoadConfigFromURL(url string) (*types.Config, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var config types.Config
	err = json.Unmarshal(body, &config)
	if err != nil {
		return &config, fmt.Errorf("error unmarshalling config: %w", err)
	}

	return setConfigDefaults(&config), nil
}

func setConfigDefaults(config *types.Config) *types.Config {
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

	return config
}

// ConfigurationOverrides creates a map of config file overrides for filenames, their keys, and values.
func ConfigurationOverrides(cfg types.Chain) map[string]any {
	var toml map[string]any

	switch cfg.ChainType {
	case "cosmos":
		toml = cosmosConfigOverride(cfg)
	default:
		toml = make(testutil.Toml, 0)
	}

	for _, o := range cfg.ConfigFileOverrides {
		for k, v := range o.Paths {

			// if o.File is empty, we only save the KV pair directly
			if o.File == "" {
				// "config_file_overrides": [
				//     {"paths": {"--load-state": "state/avs-and-eigenlayer-deployed-anvil-state.json"}}
				// ],

				toml[k] = v
				continue
			}

			// create file key if it does not exist
			if _, ok := toml[o.File]; !ok {
				toml[o.File] = make(testutil.Toml, 0)
			}

			// if there is no path, save the KV directly without the header
			if !strings.Contains(k, ".") {
				toml[o.File].(testutil.Toml)[k] = v
				continue
			}

			// separate the path and key
			path, key := strings.Split(k, ".")[0], strings.Split(k, ".")[1]

			// create path key if it does not exist
			if _, ok := toml[o.File].(testutil.Toml)[path]; !ok {
				toml[o.File].(testutil.Toml)[path] = testutil.Toml{}
			}

			// save the new KV pair
			toml[o.File].(testutil.Toml)[path].(testutil.Toml)[key] = v
		}
	}

	fmt.Println(cfg.ChainID, "Toml File", toml)
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
