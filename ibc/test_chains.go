package ibc

import (
	"testing"

	"github.com/ory/dockertest"
)

func GaiaChain(t *testing.T, pool *dockertest.Pool, home string, networkID string, numValidators int, numFullNodes int) Chain {
	return NewCosmosChain(t, pool, home, networkID, "gaia", "cosmoshub-1004", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", numValidators, numFullNodes)
}

func OsmosisChain(t *testing.T, pool *dockertest.Pool, home string, networkID string, numValidators int, numFullNodes int) Chain {
	return NewCosmosChain(t, pool, home, networkID, "osmosis", "osmosis-1001", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", numValidators, numFullNodes)
}

func JunoChain(t *testing.T, pool *dockertest.Pool, home string, networkID string, numValidators int, numFullNodes int) Chain {
	return NewCosmosChain(t, pool, home, networkID, "juno", "juno-725", "v2.3.0", "junod", "juno", "ujuno", "0.0ujuno", 1.3, "672h", numValidators, numFullNodes)
}
