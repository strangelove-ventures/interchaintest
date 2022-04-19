package test

import (
	"testing"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
	"github.com/stretchr/testify/require"
)

// These tests are run by CI

func getTestChainFactory() ibc.ChainFactory {
	return ibc.NewBuiltinChainFactory(
		[]ibc.BuiltinChainFactoryEntry{
			{Name: "gaia", Version: "v6.0.4", ChainID: "cosmoshub-1004", NumValidators: 4, NumFullNodes: 1},
			{Name: "osmosis", Version: "v7.0.4", ChainID: "osmosis-1001", NumValidators: 4, NumFullNodes: 1},
		},
	)
}

func TestRelayerByRelayerTest(t *testing.T) {
	relayertest.TestRelayer(t, getTestChainFactory(), ibc.NewBuiltinRelayerFactory(ibc.CosmosRly))
}

// queued packet with default timeout should be relayed
func TestRelayPacket(t *testing.T) {
	relayerImplementation := ibc.CosmosRly

	require.NoError(t, ibc.IBCTestCase{}.RelayPacketTest(t.Name(), getTestChainFactory(), relayerImplementation))
}

// queued packet with no timeout should be relayed
func TestNoTimeout(t *testing.T) {
	relayerImplementation := ibc.CosmosRly

	require.NoError(t, ibc.IBCTestCase{}.RelayPacketTestNoTimeout(t.Name(), getTestChainFactory(), relayerImplementation))
}

// queued packet with relative height timeout that expires should not be relayed
func TestHeightTimeout(t *testing.T) {
	relayerImplementation := ibc.CosmosRly

	require.NoError(t, ibc.IBCTestCase{}.RelayPacketTestHeightTimeout(t.Name(), getTestChainFactory(), relayerImplementation))
}

// queued packet with relative timestamp timeout (ns) that expires should not be relayed
func TestTimestampTimeout(t *testing.T) {
	t.Skip() // TODO turn this back on once fixed in cosmos relayer
	relayerImplementation := ibc.CosmosRly

	require.NoError(t, ibc.IBCTestCase{}.RelayPacketTestTimestampTimeout(t.Name(), getTestChainFactory(), relayerImplementation))
}
