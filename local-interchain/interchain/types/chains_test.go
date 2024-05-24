package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChainsGeneration(t *testing.T) {
	require.NoError(t, ChainCosmosHub().SaveJSON("chains/gen-cosmoshub.json"))
	require.NoError(t, ChainEthereum().SaveJSON("chains/gen-ethereum.json"))
	require.NoError(t, ChainJuno("localjuno-1").SaveJSON("chains/gen-juno.json"))
	require.NoError(t, ChainStargaze().SaveJSON("chains/gen-stargaze.json"))

	// Creates 2 IBC connected chains
	cc, err := ChainsIBC(ChainJuno("localjuno-1"), ChainJuno("localjuno-2"))
	require.NoError(t, err)
	require.NoError(t, cc.SaveJSON("chains/gen-juno-ibc.json"))
}
