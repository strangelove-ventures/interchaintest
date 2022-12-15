package polkadot_test

import (
	"context"
	"testing"

	ibctest "github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPolkadotComposableChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	nv := 5
	nf := 3

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			/*Name:    "composable",
			//Version: "polkadot:v0.9.19,composable:centauri",
			Version: "seunlanlege/centauri-polkadot:v0.9.27,seunlanlege/centauri-parachain:v0.9.27",
			//Version: "polkadot:v0.9.27,composable:centauri",
			ChainConfig: ibc.ChainConfig{
				ChainID: "rococo-local",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},*/
		ChainConfig: ibc.ChainConfig{
			Type: "polkadot",
			Name: "composable",
			ChainID:      "rococo-local",
			Images: []ibc.DockerImage{
				{
					Repository: "seunlanlege/centauri-polkadot",
					Version: "v0.9.27",
					UidGid: "1025:1025",
				},
				{
					Repository: "seunlanlege/centauri-parachain",
					Version: "v0.9.27",
					//UidGid: "1025:1025",
				},
			},
			Bin: "polkadot",
			Bech32Prefix: "composable",
			Denom: "uDOT",
			GasPrices: "",
			GasAdjustment: 0,
			TrustingPeriod: "",
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

	err = testutil.WaitForBlocks(ctx, 10, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")
}
