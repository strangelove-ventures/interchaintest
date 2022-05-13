package ibctest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/strangelove-ventures/ibctest/chain/penumbra"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/ibctest/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/ibc"
)

var chainConfigs = []ibc.ChainConfig{
	cosmos.NewCosmosHeighlinerChainConfig("gaia", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h"),
	cosmos.NewCosmosHeighlinerChainConfig("osmosis", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h"),
	cosmos.NewCosmosHeighlinerChainConfig("juno", "junod", "juno", "ujuno", "0.0025ujuno", 1.3, "672h"),
	penumbra.NewPenumbraChainConfig(),
}

var chainConfigMap map[string]ibc.ChainConfig

func init() {
	chainConfigMap = make(map[string]ibc.ChainConfig)
	for _, chainConfig := range chainConfigs {
		chainConfigMap[chainConfig.Name] = chainConfig
	}
}

func GetChain(testName, name, version, chainID string, numValidators, numFullNodes int, log *zap.Logger) (ibc.Chain, error) {
	chainConfig, exists := chainConfigMap[name]
	if !exists {
		return nil, fmt.Errorf("no chain configuration for %s", name)
	}

	chainConfig.ChainID = chainID

	switch chainConfig.Type {
	case "cosmos":
		chainConfig.Images[0].Version = version
		return cosmos.NewCosmosChain(testName, chainConfig, numValidators, numFullNodes, log), nil
	case "penumbra":
		versionSplit := strings.Split(version, ",")
		if len(versionSplit) != 2 {
			return nil, errors.New("penumbra version should be comma separated penumbra_version,tendermint_version")
		}
		chainConfig.Images[0].Version = versionSplit[1]
		chainConfig.Images[1].Version = versionSplit[0]
		return penumbra.NewPenumbraChain(testName, chainConfig, numValidators, numFullNodes), nil
	default:
		return nil, fmt.Errorf("unexpected error, unknown chain type: %s for chain: %s", chainConfig.Type, name)
	}
}
