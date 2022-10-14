package penumbra_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest/v3"
	"github.com/strangelove-ventures/ibctest/v3/ibc"
	"github.com/strangelove-ventures/ibctest/v3/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	nv := 4

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "025-helike,v0.35.9",
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

	err = chain.Initialize(ctx, t.Name(), client, network)
	require.NoError(t, err, "failed to initialize penumbra chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start penumbra chain")

	err = test.WaitForBlocks(ctx, 10, chain)

	require.NoError(t, err, "penumbra chain failed to make blocks")
}
