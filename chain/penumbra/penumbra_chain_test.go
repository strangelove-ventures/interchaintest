package penumbra_test

import (
	"testing"

	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/stretchr/testify/require"
)

func TestPenumbraChainStart(t *testing.T) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	chain, err := ibctest.GetChain(t.Name(), "penumbra", "009-cyllene,v0.35.0", "penumbra-cyllene", 4, 1)
	require.NoError(t, err, "failed to get penumbra chain")

	err = chain.Initialize(t.Name(), home, pool, network)
	require.NoError(t, err, "failed to initialize penumbra chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start penumbra chain")

	_, err = chain.WaitForBlocks(50)
	require.NoError(t, err, "penumbra chain failed to make blocks")
}
