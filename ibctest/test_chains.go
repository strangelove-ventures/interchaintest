package ibctest

import (
	"fmt"

	"github.com/strangelove-ventures/ibc-test-framework/chain/cosmos"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
)

var chainConfigs = []ibc.ChainConfig{
	cosmos.NewCosmosHeighlinerChainConfig("gaia", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h"),
	cosmos.NewCosmosHeighlinerChainConfig("osmosis", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h"),
	cosmos.NewCosmosHeighlinerChainConfig("juno", "junod", "juno", "ujuno", "0.0025ujuno", 1.3, "672h"),
}

var chainConfigMap map[string]ibc.ChainConfig

func init() {
	chainConfigMap = make(map[string]ibc.ChainConfig)
	for _, chainConfig := range chainConfigs {
		chainConfigMap[chainConfig.Name] = chainConfig
	}
}

func GetChain(testName, name, version, chainID string, numValidators, numFullNodes int) (ibc.Chain, error) {
	chainConfig, exists := chainConfigMap[name]
	if !exists {
		return nil, fmt.Errorf("No chain configuration for %s", name)
	}
	chainConfig.Version = version
	chainConfig.ChainID = chainID

	switch chainConfig.Type {
	case "cosmos":
		return cosmos.NewCosmosChain(testName, chainConfig, numValidators, numFullNodes), nil
	default:
		return nil, fmt.Errorf("unexpected error, unknown chain type: %s for chain: %s", chainConfig.Type, name)
	}
}
