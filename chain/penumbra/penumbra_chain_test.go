package penumbra_test

import (
	"context"
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

	pool, network := ibctest.DockerSetup(t)
	home := t.TempDir() // Must be before chain cleanup to avoid test error during cleanup.

	log := zap.NewNop()
	chain, err := ibctest.BuiltinChainFactoryEntry{
		Name:          "penumbra",
		Version:       "015-ersa-v2,v0.35.4",
		ChainID:       "penumbra-1",
		NumValidators: 4,
		NumFullNodes:  1,
	}.GetChain(log, t.Name())
	require.NoError(t, err, "failed to get penumbra chain")

	ctx := context.Background()
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
