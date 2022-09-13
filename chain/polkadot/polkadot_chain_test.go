package polkadot_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPolkadotComposableChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	nv := 5
	nf := 3

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:    "composable",
			Version: "polkadot:v0.9.19,composable:v2.1.9",
			ChainConfig: ibc.ChainConfig{
				ChainID: "rococo-local",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get polkadot chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	ctx := context.Background()

	err = chain.Initialize(ctx, t.Name(), client, network)
	require.NoError(t, err, "failed to initialize polkadot chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start polkadot chain")

	err = test.WaitForBlocks(ctx, 10, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")
}
