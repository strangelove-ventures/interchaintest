package test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
)

// These tests are run by CI

func getTestChainFactory(logger log.Logger) ibctest.ChainFactory {
	return ibctest.NewBuiltinChainFactory(
		[]ibctest.BuiltinChainFactoryEntry{
			{Name: "gaia", Version: "v7.0.1", ChainID: "cosmoshub-1004", NumValidators: 2, NumFullNodes: 1},
			{Name: "osmosis", Version: "v7.2.0", ChainID: "osmosis-1001", NumValidators: 2, NumFullNodes: 1},
		},
		logger,
	)
}

func TestRelayer(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	if testing.Short() {
		t.Skip()
	}

	logger := log.New(os.Stderr, "console", "info")
	relayertest.TestRelayer(t, getTestChainFactory(logger), ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, logger))
}
