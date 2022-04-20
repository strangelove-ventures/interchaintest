package test

import (
	"testing"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
)

// These tests are run by CI

func getTestChainFactory() ibc.ChainFactory {
	return ibc.NewBuiltinChainFactory(
		[]ibc.BuiltinChainFactoryEntry{
			{Name: "gaia", Version: "v6.0.4", ChainID: "cosmoshub-1004", NumValidators: 1, NumFullNodes: 1},
			{Name: "osmosis", Version: "v7.0.4", ChainID: "osmosis-1001", NumValidators: 1, NumFullNodes: 1},
		},
	)
}

func TestRelayer(t *testing.T) {
	relayertest.TestRelayer(t, getTestChainFactory(), ibc.NewBuiltinRelayerFactory(ibc.CosmosRly))
}
