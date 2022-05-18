package penumbra_test

import (
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	log := zap.NewNop()
	chain, err := ibctest.GetChain(t.Name(), "penumbra", "015-ersa-v2,v0.35.4", "penumbra-1", 4, 1, log)
	require.NoError(t, err, "failed to get penumbra chain")

	t.Cleanup(func() {
		if err := chain.Cleanup(ctx); err != nil {
			log.Warn("Chain cleanup failed", zap.String("chain", chain.Config().ChainID), zap.Error(err))
		}
	})

	err = chain.Initialize(t.Name(), home, pool, network)
	require.NoError(t, err, "failed to initialize penumbra chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start penumbra chain")

	err = test.WaitForBlocks(ctx, 10, chain)

	require.NoError(t, err, "penumbra chain failed to make blocks")
}
