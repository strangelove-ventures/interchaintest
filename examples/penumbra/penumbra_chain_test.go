package penumbra_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"

	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
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

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name: "penumbra",
			// Version: "040-themisto.1,v0.34.23",
			Version: "045-metis,v0.34.23",
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

	initBalance := math.NewInt(1_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chain)
	require.Equal(t, 1, len(users))

	alice := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chain)
	require.NoError(t, err)

	aliceBal, err := chain.GetBalance(ctx, alice.KeyName(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))

	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chain)
	require.Equal(t, 1, len(users))

	bob := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chain)
	require.NoError(t, err)

	bobBal, err := chain.GetBalance(ctx, bob.KeyName(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, initBalance))

	bobAddr, err := chain.GetAddress(ctx, bob.KeyName())
	require.NoError(t, err)

	transfer := ibc.WalletAmount{
		Address: string(bobAddr),
		Denom:   chain.Config().Denom,
		Amount:  math.NewInt(1_000),
	}

	err = chain.SendFunds(ctx, alice.KeyName(), transfer)
	require.NoError(t, err)

	/*
		TODO:
		without this sleep statement we see intermittent failures where we will observe the tokens taken from alice's balance
		but not added to bob's balance. after debugging it seems like this is because alice's client is in sync but bob's is not.
		we may need a way to check if each client is in sync before making any assertions about chain state after some state transition.
		alternatively, we may want to wrap penumbra related queries in a retry.
	*/
	time.Sleep(1 * time.Second)

	aliceNewBal, err := chain.GetBalance(ctx, alice.KeyName(), chain.Config().Denom)
	require.NoError(t, err)

	bobNewBal, err := chain.GetBalance(ctx, bob.KeyName(), chain.Config().Denom)
	require.NoError(t, err)

	aliceExpected := aliceBal.Sub(transfer.Amount)
	bobExpected := bobBal.Add(transfer.Amount)

	require.True(t, aliceNewBal.Equal(aliceExpected), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceNewBal, aliceExpected))
	require.True(t, bobNewBal.Equal(bobExpected), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobNewBal, bobExpected))
}
