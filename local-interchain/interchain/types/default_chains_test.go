package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChains(t *testing.T) {
	require.NoError(t, ChainCosmosHub().SaveJSON("chains/generated-ethereum.json"))
	require.NoError(t, ChainEthereum().SaveJSON("chains/generated-ethereum.json"))
}
