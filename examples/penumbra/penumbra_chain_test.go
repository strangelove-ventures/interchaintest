package penumbra_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
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
	fn := 0

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "v0.58.0,v0.34.24",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbra-1",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
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

	err = testutil.WaitForBlocks(ctx, 30, chain)

	require.NoError(t, err, "penumbra chain failed to make blocks")

	alice := chain.(*penumbra.PenumbraChain).PenumbraNodes[0]
	bob := chain.(*penumbra.PenumbraChain).PenumbraNodes[1]

	aliceBal, err := chain.GetBalance(ctx, alice.PenumbraClientNodes["validator"].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	bobBal, err := chain.GetBalance(ctx, bob.PenumbraClientNodes["validator"].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	// TODO: genesis allocations should be configurable, right now we are using a hardcoded value in PenumbraChain.Start
	expectedBal := math.NewInt(1_000_000_000_000)

	t.Logf("Alice Balance: %s \n", aliceBal)
	t.Logf("Bob Balance: %s \n", bobBal)

	require.True(t, aliceBal.Equal(expectedBal))
	require.True(t, bobBal.Equal(expectedBal))

	transfer := ibc.WalletAmount{
		Address: bob.PenumbraClientNodes["validator"].KeyName,
		Denom:   chain.Config().Denom,
		Amount:  math.NewInt(1_000),
	}

	err = chain.SendFunds(ctx, alice.PenumbraClientNodes["validator"].KeyName, transfer)
	require.NoError(t, err)

	aliceNewBal, err := chain.GetBalance(ctx, alice.PenumbraClientNodes["validator"].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	bobNewBal, err := chain.GetBalance(ctx, bob.PenumbraClientNodes["validator"].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	t.Logf("Alice Balance: %s \n", aliceNewBal)
	t.Logf("Bob Balance: %s \n", bobNewBal)

	require.True(t, aliceNewBal.Equal(aliceBal.Sub(transfer.Amount)))
	require.True(t, bobNewBal.Equal(bobBal.Add(transfer.Amount)))
}
