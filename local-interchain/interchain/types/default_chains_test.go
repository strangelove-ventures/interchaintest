package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChains(t *testing.T) {
	require.NoError(t, ChainCosmosHub().SaveJSON("chains/gen-cosmoshub.json"))
	require.NoError(t, ChainEthereum().SaveJSON("chains/gen-ethereum.json"))
}
