package penumbra_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, _, network := ibctest.DockerSetup(t)
	home := ibctest.TempDir(t) // Must be before chain cleanup to avoid test error during cleanup.

	nv := 4

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "015-ersa-v2,v0.35.4",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbra-1",
			},
			NumValidators: &nv,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get penumbra chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	ctx := context.Background()
	t.Cleanup(func() {
		if err := chain.Cleanup(ctx); err != nil {
			t.Logf("Chain cleanup for %s failed: %v", chain.Config().ChainID, err)
		}
	})

	err = chain.Initialize(t.Name(), home, client, network)
	require.NoError(t, err, "failed to initialize penumbra chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start penumbra chain")

	err = test.WaitForBlocks(ctx, 10, chain)

	require.NoError(t, err, "penumbra chain failed to make blocks")
}
