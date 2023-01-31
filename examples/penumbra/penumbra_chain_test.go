package penumbra_test

import (
	"context"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v3"
	"github.com/strangelove-ventures/interchaintest/v3/ibc"
	"github.com/strangelove-ventures/interchaintest/v3/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 4

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "033-eirene,v0.34.21",
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

	err = testutil.WaitForBlocks(ctx, 10, chain)

	require.NoError(t, err, "penumbra chain failed to make blocks")
}
