package polkadot_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/polkadot"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPolkadotComposableChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nv := 5
	nf := 3

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainConfig: ibc.ChainConfig{
				Type:    "polkadot",
				Name:    "composable",
				ChainID: "rococo-local",
				Images: []ibc.DockerImage{
					{
						Repository: "seunlanlege/centauri-polkadot",
						Version:    "v0.9.27",
						UidGid:     "1000:1000",
					},
					{
						Repository: "seunlanlege/centauri-parachain",
						Version:    "v0.9.27",
						//UidGid: "1025:1025",
					},
				},
				Bin:            "polkadot",
				Bech32Prefix:   "composable",
				Denom:          "uDOT",
				GasPrices:      "",
				GasAdjustment:  0,
				TrustingPeriod: "",
				CoinType:       "354",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get polkadot chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))

	polkadotChain := chain.(*polkadot.PolkadotChain)

	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")

	PARACHAIN_DEFAULT_AMOUNT := math.NewInt(1_152_921_504_606_847_000)
	RELAYCHAIN_DEFAULT_AMOUNT := math.NewInt(1_100_000_000_000_000_000)
	FAUCET_AMOUNT := math.NewInt(100_000_000_000_000_000) // set in interchain.go/global
	//RELAYER_AMOUNT :=                   1_000_000_000_000 // set in interchain.go/global

	// Check the faucet amounts
	polkadotFaucetAddress, err := polkadotChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)

	polkadotFaucetAmount, err := polkadotChain.GetBalance(ctx, string(polkadotFaucetAddress), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot faucet amount: ", polkadotFaucetAmount)
	require.True(t, polkadotFaucetAmount.Equal(FAUCET_AMOUNT), "Polkadot faucet amount not expected")

	parachainFaucetAmount, err := polkadotChain.GetBalance(ctx, string(polkadotFaucetAddress), "")
	require.NoError(t, err)
	fmt.Println("Parachain faucet amount: ", parachainFaucetAmount)
	require.True(t, parachainFaucetAmount.Equal(FAUCET_AMOUNT), "Parachain faucet amount not expected")

	// Check alice
	polkadotAliceAddress, err := polkadotChain.GetAddress(ctx, "alice")
	require.NoError(t, err)
	polkadotAliceAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceAddress), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot alice amount: ", polkadotAliceAmount)
	require.True(t, polkadotAliceAmount.Equal(RELAYCHAIN_DEFAULT_AMOUNT), "Relaychain alice amount not expected")

	parachainAliceAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceAddress), "")
	require.NoError(t, err)
	fmt.Println("Parachain alice amount: ", parachainAliceAmount)
	require.True(t, parachainAliceAmount.Equal(PARACHAIN_DEFAULT_AMOUNT), "Parachain alice amount not expected")

	// Check alice stash
	polkadotAliceStashAddress, err := polkadotChain.GetAddress(ctx, "alicestash")
	require.NoError(t, err)
	polkadotAliceStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceStashAddress), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot alice stash amount: ", polkadotAliceStashAmount)
	require.True(t, polkadotAliceStashAmount.Equal(RELAYCHAIN_DEFAULT_AMOUNT), "Relaychain alice stash amount not expected")

	parachainAliceStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceStashAddress), "")
	require.NoError(t, err)
	fmt.Println("Parachain alice stash amount: ", parachainAliceStashAmount)
	require.True(t, parachainAliceStashAmount.Equal(PARACHAIN_DEFAULT_AMOUNT), "Parachain alice stash amount not expected")

	// Check bob
	polkadotBobAddress, err := polkadotChain.GetAddress(ctx, "bob")
	require.NoError(t, err)
	polkadotBobAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobAddress), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot bob amount: ", polkadotBobAmount)
	require.True(t, polkadotBobAmount.Equal(RELAYCHAIN_DEFAULT_AMOUNT), "Relaychain bob amount not expected")

	parachainBobAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobAddress), "")
	require.NoError(t, err)
	fmt.Println("Parachain bob amount: ", parachainBobAmount)
	require.True(t, parachainBobAmount.Equal(PARACHAIN_DEFAULT_AMOUNT), "Parachain bob amount not expected")

	// Check bob stash
	polkadotBobStashAddress, err := polkadotChain.GetAddress(ctx, "bobstash")
	require.NoError(t, err)
	polkadotBobStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobStashAddress), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot bob stash amount: ", polkadotBobStashAmount)
	require.True(t, polkadotBobStashAmount.Equal(RELAYCHAIN_DEFAULT_AMOUNT), "Relaychain bob stash amount not expected")

	parachainBobStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobStashAddress), "")
	require.NoError(t, err)
	fmt.Println("Parachain bob stash amount: ", parachainBobStashAmount)
	require.True(t, parachainBobStashAmount.Equal(PARACHAIN_DEFAULT_AMOUNT), "Parachain bob stash amount not expected")

	// Fund user1 on both relay and parachain, must wait a block to fund user2 due to same faucet address
	fundAmount := math.NewInt(12_333_000_000_000)
	users1 := interchaintest.GetAndFundTestUsers(t, ctx, "user1", fundAmount.Int64(), polkadotChain)
	user1 := users1[0]
	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")

	// Fund user2 on both relay and parachain, check that user1 was funded properly
	users2 := interchaintest.GetAndFundTestUsers(t, ctx, "user2", fundAmount.Int64(), polkadotChain)
	user2 := users2[0]
	polkadotUser1Amount, err := polkadotChain.GetBalance(ctx, user1.FormattedAddress(), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot user1 amount: ", polkadotUser1Amount)
	require.True(t, polkadotUser1Amount.Equal(fundAmount), "Initial polkadot user1 amount not expected")

	parachainUser1Amount, err := polkadotChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Parachain user1 amount: ", parachainUser1Amount)
	require.True(t, parachainUser1Amount.Equal(fundAmount), "Initial parachain user1 amount not expected")

	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")

	// Check that user2 was funded properly
	polkadotUser2Amount, err := polkadotChain.GetBalance(ctx, user2.FormattedAddress(), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot user2 amount: ", polkadotUser2Amount)
	require.True(t, polkadotUser2Amount.Equal(fundAmount), "Initial polkadot user2 amount not expected")

	parachainUser2Amount, err := polkadotChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Parachain user2 amount: ", parachainUser2Amount)
	require.True(t, parachainUser2Amount.Equal(fundAmount), "Initial parachain user2 amount not expected")

	// Transfer 1T units from user1 to user2 on both chains
	txAmount := math.NewInt(1_000_000_000_000)
	polkadotTxUser1ToUser2 := ibc.WalletAmount{
		Address: user2.FormattedAddress(),
		Amount:  txAmount,
		Denom:   polkadotChain.Config().Denom,
	}
	parachainTxUser1ToUser2 := ibc.WalletAmount{
		Address: user2.FormattedAddress(),
		Amount:  txAmount,
		Denom:   "", // Anything other than polkadot denom
	}
	err = polkadotChain.SendFunds(ctx, user1.KeyName(), polkadotTxUser1ToUser2)
	require.NoError(t, err)
	err = polkadotChain.SendFunds(ctx, user1.KeyName(), parachainTxUser1ToUser2)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")

	// Verify user1 and user2 funds on both chains are correct
	polkadotUser1Amount, err = polkadotChain.GetBalance(ctx, user1.FormattedAddress(), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot user1 amount: ", polkadotUser1Amount)
	require.True(t, polkadotUser1Amount.LTE(fundAmount.Sub(txAmount)), "Final polkadot user1 amount not expected")

	polkadotUser2Amount, err = polkadotChain.GetBalance(ctx, user2.FormattedAddress(), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot user2 amount: ", polkadotUser2Amount)
	require.True(t, fundAmount.Add(txAmount).Equal(polkadotUser2Amount), "Final polkadot user2 amount not expected")

	parachainUser1Amount, err = polkadotChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Parachain user1 amount: ", parachainUser1Amount)
	require.True(t, parachainUser1Amount.LTE(fundAmount.Sub(txAmount)), "Final parachain user1 amount not expected")

	parachainUser2Amount, err = polkadotChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Parachain user2 amount: ", parachainUser2Amount)
	require.True(t, fundAmount.Add(txAmount).Equal(parachainUser2Amount), "Final parachain user2 amount not expected")
}
