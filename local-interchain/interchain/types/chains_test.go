package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChainsGeneration(t *testing.T) {
	t.Run("single chain runs", func(t *testing.T) {
		require.NoError(t, ChainEthereum().SaveJSON("chains/gen-ethereum.json"))
		require.NoError(t, ChainStargaze().SaveJSON("chains/gen-stargaze.json"))
		require.NoError(t, ChainCosmosHub("localcosmos-1").SaveJSON("chains/gen-cosmoshub.json"))
		require.NoError(t, ChainOsmosis().SaveJSON("chains/gen-osmosis.json"))
		require.NoError(t, ChainJuno("localjuno-1").SaveYAML("chains/gen-juno.yml"))
	})

	t.Run("2 IBC connected chains", func(t *testing.T) {
		j1, j2 := ChainJuno("localjuno-1"), ChainJuno("localjuno-2")
		j1.SetAppendedIBCPathLink(j2)

		require.NoError(t, NewChainsConfig(j1, j2).SaveJSON("chains/gen-juno-ibc.json"))
	})

	t.Run("4 way IBC setup", func(t *testing.T) {
		hub := ChainCosmosHub("localhub-1")
		hub2 := ChainCosmosHub("localhub-2")
		juno1 := ChainJuno("localjuno-1")
		osmo1 := ChainOsmosis()

		hub.SetAppendedIBCPathLink(hub2).SetAppendedIBCPathLink(juno1)
		hub2.SetAppendedIBCPathLink(juno1)
		osmo1.SetAppendedIBCPathLink(hub).SetAppendedIBCPathLink(juno1)

		require.NoError(t, NewChainsConfig(hub, hub2, juno1, osmo1).SaveJSON("chains/gen-4-ibc.json"))
	})
}
