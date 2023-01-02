package polkadot_test

import (
	"context"
	"fmt"
	"testing"

	ibctest "github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/polkadot"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
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

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nv := 5
	nf := 3

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
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
			CoinType: "354",
		},
		NumValidators: &nv,
		NumFullNodes:  &nf,
	},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get polkadot chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	ic := ibctest.NewInterchain().
	AddChain(chain)

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))	

	polkadotChain := chain.(*polkadot.PolkadotChain)

	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "polkadot chain failed to make blocks")

	PARACHAIN_DEFAULT_AMOUNT :=   1_152_921_504_606_847_000
	RELAYCHAIN_DEFAULT_AMOUNT :=  1_100_000_000_000_000_000
	FAUCET_AMOUNT :=                    100_000_000_000_000 // set in interchain.go/global
	//RELAYER_AMOUNT :=                   1_000_000_000_000 // set in interchain.go/global

	// Check the faucet amounts
	polkadotFaucetAddress, err := polkadotChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)
	polkadotFaucetAmount, err := polkadotChain.GetBalance(ctx, string(polkadotFaucetAddress), "")
	require.NoError(t, err)
	fmt.Println("Polkadot faucet amount: ", polkadotFaucetAmount)
	require.Equal(t, int64(FAUCET_AMOUNT), polkadotFaucetAmount, "Polkadot faucet amount not expected")
	parachainFaucetAmount, err := polkadotChain.ParachainNodes[0][0].GetBalance(string(polkadotFaucetAddress))
	require.NoError(t, err)
	fmt.Println("Parachain faucet amount: ", parachainFaucetAmount)
	require.Equal(t, int64(FAUCET_AMOUNT), parachainFaucetAmount, "Parachain faucet amount not expected")

	// Check alice
	polkadotAliceAddress, err := polkadotChain.GetAddress(ctx, "alice")
	require.NoError(t, err)
	polkadotAliceAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceAddress), "")
	require.NoError(t, err)
	fmt.Println("Polkadot alice amount: ", polkadotAliceAmount)
	require.Equal(t, int64(RELAYCHAIN_DEFAULT_AMOUNT), polkadotAliceAmount, "Relaychain alice amount not expected")
	parachainAliceAmount, err := polkadotChain.ParachainNodes[0][0].GetBalance(string(polkadotAliceAddress))
	require.NoError(t, err)
	fmt.Println("Parachain alice amount: ", parachainAliceAmount)
	require.Equal(t, int64(PARACHAIN_DEFAULT_AMOUNT), parachainAliceAmount, "Parachain alice amount not expected")

	// Check alice stash
	polkadotAliceStashAddress, err := polkadotChain.GetAddress(ctx, "alicestash")
	require.NoError(t, err)
	polkadotAliceStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotAliceStashAddress), "")
	require.NoError(t, err)
	fmt.Println("Polkadot alice stash amount: ", polkadotAliceStashAmount)
	require.Equal(t, int64(RELAYCHAIN_DEFAULT_AMOUNT), polkadotAliceStashAmount, "Relaychain alice stash amount not expected")
	parachainAliceStashAmount, err := polkadotChain.ParachainNodes[0][0].GetBalance(string(polkadotAliceStashAddress))
	require.NoError(t, err)
	fmt.Println("Parachain alice stash amount: ", parachainAliceStashAmount)
	require.Equal(t, int64(PARACHAIN_DEFAULT_AMOUNT), parachainAliceStashAmount, "Parachain alice stash amount not expected")

	// Check bob
	polkadotBobAddress, err := polkadotChain.GetAddress(ctx, "bob")
	require.NoError(t, err)
	polkadotBobAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobAddress), "")
	require.NoError(t, err)
	fmt.Println("Polkadot bob amount: ", polkadotBobAmount)
	require.Equal(t, int64(RELAYCHAIN_DEFAULT_AMOUNT), polkadotBobAmount, "Relaychain bob amount not expected")
	parachainBobAmount, err := polkadotChain.ParachainNodes[0][0].GetBalance(string(polkadotBobAddress))
	require.NoError(t, err)
	fmt.Println("Parachain bob amount: ", parachainBobAmount)
	require.Equal(t, int64(PARACHAIN_DEFAULT_AMOUNT), parachainBobAmount, "Parachain bob amount not expected")

	// Check bob stash
	polkadotBobStashAddress, err := polkadotChain.GetAddress(ctx, "bobstash")
	require.NoError(t, err)
	polkadotBobStashAmount, err := polkadotChain.GetBalance(ctx, string(polkadotBobStashAddress), "")
	require.NoError(t, err)
	fmt.Println("Polkadot bob stash amount: ", polkadotBobStashAmount)
	require.Equal(t, int64(RELAYCHAIN_DEFAULT_AMOUNT), polkadotBobStashAmount, "Relaychain bob stash amount not expected")
	parachainBobStashAmount, err := polkadotChain.ParachainNodes[0][0].GetBalance(string(polkadotBobStashAddress))
	require.NoError(t, err)
	fmt.Println("Parachain bob stash amount: ", parachainBobStashAmount)
	require.Equal(t, int64(PARACHAIN_DEFAULT_AMOUNT), parachainBobStashAmount, "Parachain bob stash amount not expected")

}
