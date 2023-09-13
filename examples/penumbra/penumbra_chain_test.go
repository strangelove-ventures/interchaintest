package penumbra_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
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

	const aliceKeyName = "validator-0"
	const bobKeyName = "validator-1"

	aliceBal, err := chain.GetBalance(ctx, alice.PenumbraClientNodes[aliceKeyName].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	bobBal, err := chain.GetBalance(ctx, bob.PenumbraClientNodes[bobKeyName].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	// TODO: genesis allocations should be configurable, right now we are using a hardcoded value in PenumbraChain.Start
	expectedBal := math.NewInt(1_000_000_000_000)

	require.True(t, aliceBal.Equal(expectedBal))
	require.True(t, bobBal.Equal(expectedBal))

	bobAddr, err := chain.GetAddress(ctx, bob.PenumbraClientNodes[bobKeyName].KeyName)
	require.NoError(t, err)

	transfer := ibc.WalletAmount{
		Address: string(bobAddr),
		Denom:   chain.Config().Denom,
		Amount:  math.NewInt(1_000),
	}

	err = chain.SendFunds(ctx, alice.PenumbraClientNodes[aliceKeyName].KeyName, transfer)
	require.NoError(t, err)

	/*
		TODO:
		without this sleep statement we see intermittent failures where we will observe the tokens taken from alice's balance
		but not added to bob's balance. after debugging it seems like this is because alice's client is in sync but bob's is not.
		we may need a way to check if each client is in sync before making any assertions about chain state after some state transition.
		alternatively, we may wrap penumbra related queries in a retry.
	*/
	time.Sleep(1 * time.Second)

	aliceNewBal, err := chain.GetBalance(ctx, alice.PenumbraClientNodes[aliceKeyName].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	bobNewBal, err := chain.GetBalance(ctx, bob.PenumbraClientNodes[bobKeyName].KeyName, chain.Config().Denom)
	require.NoError(t, err)

	aliceExpected := aliceBal.Sub(transfer.Amount)
	bobExpected := bobBal.Add(transfer.Amount)

	require.True(t, aliceNewBal.Equal(aliceExpected), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceNewBal, aliceExpected))
	require.True(t, bobNewBal.Equal(bobExpected), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobNewBal, bobExpected))
}
