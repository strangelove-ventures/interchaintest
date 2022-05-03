package test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
)

// These tests are run by CI

func getTestChainFactory() ibctest.ChainFactory {
	return ibctest.NewBuiltinChainFactory(
		[]ibctest.BuiltinChainFactoryEntry{
			{Name: "gaia", Version: "v7.0.1", ChainID: "cosmoshub-1004", NumValidators: 2, NumFullNodes: 1},
			{Name: "osmosis", Version: "v7.2.0", ChainID: "osmosis-1001", NumValidators: 2, NumFullNodes: 1},
		},
	)
}

func TestRelayer(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	if testing.Short() {
		t.Skip()
	}

	logfile, err := ibctest.CreateLogFile("test-relayer.log")
	if err != nil {
		t.Fatal("failed to create log file:", err)
	}
	t.Cleanup(func() { _ = logfile.Close() })

	logger := log.New(logfile, log.Console, log.InfoLevel)

	t.Logf("View chain and relayer logs at: %s", logfile.Name())
	relayertest.TestRelayer(t, getTestChainFactory(), ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, logger))
}
