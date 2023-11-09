package penumbra_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestPenumbraNetworkIntegration exercises various facilities of pclientd and is used as a basic integration test
// to assert that interacting with a local Penumbra testnet via pclientd works as intended.
//
// This test case is ported from a Rust integration test found in the Penumbra repo at the link below:
// https://github.com/penumbra-zone/penumbra/blob/45bdbbeefc2f0d3ebf09e2f37d0545d8b1e094d8/crates/bin/pclientd/tests/network_integration.rs
func TestPenumbraNetworkIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 2
	fn := 0

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "v0.60.0,v0.34.24",
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
	chain := chains[0].(*penumbra.PenumbraChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	rep := testreporter.NewNopReporter()

	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))

	t.Cleanup(func() {
		err := ic.Close()
		if err != nil {
			panic(err)
		}
	})

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
