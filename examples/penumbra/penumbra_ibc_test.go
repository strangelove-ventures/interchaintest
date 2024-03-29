package penumbra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestPenumbraToPenumbraIBC asserts that basic IBC functionality works between two Penumbra testnet networks.
// Two instances of Penumbra will be spun up, the go-relayer will be configured, and an ics-20 token transfer will be
// conducted from chainA -> chainB.
func TestPenumbraToPenumbraIBC(t *testing.T) {
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
			Version: "v0.71.0,v0.37.5",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbraA-0",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
		{
			Name:    "penumbra",
			Version: "v0.71.0,v0.37.5",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbraB-0",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get penumbra chains")
	require.Len(t, chains, 2)

	chainA := chains[0].(*penumbra.PenumbraChain)
	chainB := chains[1].(*penumbra.PenumbraChain)

	i := ibc.DockerImage{
		Repository: "relayer",
		Version:    "local",
		UidGid:     "1025:1025",
	}
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.DockerImage(&i),
		relayer.ImagePull(false),
	).Build(t, client, network)

	const pathName = "ab"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:            chainA,
			Chain2:            chainB,
			Relayer:           r,
			Path:              pathName,
			CreateClientOpts:  ibc.CreateClientOptions{},
			CreateChannelOpts: ibc.CreateChannelOptions{},
		})

	ctx := context.Background()
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		err := ic.Close()
		if err != nil {
			panic(err)
		}
	})

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// Fund users and check init balances
	initBalance := math.NewInt(1_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chainA)
	require.Equal(t, 1, len(users))

	alice := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	aliceBal, err := chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))

	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chainB)
	require.Equal(t, 1, len(users))

	bob := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	bobBal, err := chainB.GetBalance(ctx, bob.KeyName(), chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, initBalance))

	// Compose ics-20 transfer details and initialize transfer
	bobAddr, err := chainB.GetAddress(ctx, bob.KeyName())
	require.NoError(t, err)

	transferAmount := math.NewInt(1_000_000)
	transfer := ibc.WalletAmount{
		Address: string(bobAddr),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	abChan, err := ibc.GetTransferChannel(ctx, r, eRep, chainA.Config().ChainID, chainB.Config().ChainID)
	require.NoError(t, err)

	h, err := chainA.Height(ctx)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, alice.KeyName(), transfer, ibc.TransferOptions{
		Timeout: &ibc.IBCTimeout{
			NanoSeconds: uint64((time.Duration(30000000) * time.Minute).Nanoseconds()),
			Height:      h + 50,
		},
		Memo: "",
	})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	expectedBal := initBalance.Sub(transferAmount)
	aliceBal, err = chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(expectedBal), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, expectedBal))

	// Compose IBC token denom information for Chain A's native token denom represented on Chain B
	ibcDenom := transfertypes.GetPrefixedDenom(abChan.Counterparty.PortID, abChan.Counterparty.ChannelID, chainA.Config().Denom)

	bobBal, err = chainB.GetBalance(ctx, bob.KeyName(), ibcDenom)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(transferAmount), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, transferAmount))
	//
	//transfer = ibc.WalletAmount{
	//	Address: bob.FormattedAddress(),
	//	Denom:   chainA.Config().Denom,
	//	Amount:  transferAmount,
	//}
	//
	//h, err = chainB.Height(ctx)
	//require.NoError(t, err)
	//
	//_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, alice.KeyName(), transfer, ibc.TransferOptions{
	//	Timeout: &ibc.IBCTimeout{
	//		NanoSeconds: uint64((time.Duration(3) * time.Second).Nanoseconds()),
	//		Height:      h + 5,
	//	},
	//	Memo: "",
	//})
	//require.NoError(t, err)
	//
	//err = testutil.WaitForBlocks(ctx, 7, chainA)
	//require.NoError(t, err)
	//
	//aliceBal, err = chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	//require.NoError(t, err)
	//require.True(t, aliceBal.Equal(expectedBal), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))
	//
	//bobBal, err = chainB.GetBalance(ctx, bob.FormattedAddress(), ibcDenom)
	//require.NoError(t, err)
	//require.True(t, bobBal.Equal(transferAmount), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, math.ZeroInt()))
}

// TestPenumbraToPenumbraIBC asserts that basic IBC functionality works between Penumbra and Cosmos testnet networks.
// An instance of Penumbra and a basic Cosmos network will be spun up, the go-relayer will be configured,
// and an ics-20 token transfer will be conducted from Penumbra -> Cosmos and then back from Cosmos -> Penumbra.
func TestPenumbraToCosmosIBC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 1
	fn := 0

	image := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/ibc-go-simd",
		Version:    "v8.0.0",
		UidGid:     "100:1000",
	}

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "v0.71.0,v0.37.5",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbraA-0",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
		{
			Name:          "ibc-go-simd",
			ChainName:     "simd",
			Version:       "main",
			NumValidators: &nv,
			NumFullNodes:  &fn,
			ChainConfig: ibc.ChainConfig{
				Type:          "cosmos",
				Name:          "simd",
				ChainID:       "cosmos-0",
				Images:        []ibc.DockerImage{image},
				Bin:           "simd",
				Bech32Prefix:  "cosmos",
				Denom:         "stake",
				CoinType:      "118",
				GasPrices:     "0.0stake",
				GasAdjustment: 1.1,
			},
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get chains")
	require.Len(t, chains, 2)

	chainA := chains[0].(*penumbra.PenumbraChain)
	chainB := chains[1]

	i := ibc.DockerImage{
		Repository: "relayer",
		Version:    "local",
		UidGid:     "1025:1025",
	}
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.DockerImage(&i),
		relayer.ImagePull(false),
	).Build(t, client, network)

	const pathName = "ab"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:            chainA,
			Chain2:            chainB,
			Relayer:           r,
			Path:              pathName,
			CreateClientOpts:  ibc.CreateClientOptions{},
			CreateChannelOpts: ibc.CreateChannelOptions{},
		})

	ctx := context.Background()
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		err := ic.Close()
		if err != nil {
			panic(err)
		}
	})

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// Fund users and check init balances
	initBalance := math.NewInt(1_000_000_000)
	pUsers := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chainA)
	require.Equal(t, 1, len(pUsers))

	alice := pUsers[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	aliceBal, err := chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))

	cUsers := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chainB)
	require.Equal(t, 1, len(cUsers))

	bob := cUsers[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	bobBal, err := chainB.GetBalance(ctx, bob.FormattedAddress(), chainB.Config().Denom)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, initBalance))

	// Compose ics-20 transfer details and initialize transfer
	transferAmount := math.NewInt(1_000_000)
	transfer := ibc.WalletAmount{
		Address: bob.FormattedAddress(),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	abChan, err := ibc.GetTransferChannel(ctx, r, eRep, chainA.Config().ChainID, chainB.Config().ChainID)
	require.NoError(t, err)

	h, err := chainB.Height(ctx)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, alice.KeyName(), transfer, ibc.TransferOptions{
		Timeout: &ibc.IBCTimeout{
			NanoSeconds: uint64((time.Duration(30000000) * time.Minute).Nanoseconds()),
			Height:      h + 50,
		},
		Memo: "",
	})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	expectedBal := initBalance.Sub(transferAmount)
	aliceBal, err = chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(expectedBal), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, expectedBal))

	// Compose IBC token denom information for Chain A's native token denom represented on Chain B
	ibcDenom := transfertypes.GetPrefixedDenom(abChan.Counterparty.PortID, abChan.Counterparty.ChannelID, chainA.Config().Denom)
	ibcDenomTrace := transfertypes.ParseDenomTrace(ibcDenom)
	chainADenomOnChainB := ibcDenomTrace.IBCDenom()

	bobBal, err = chainB.GetBalance(ctx, bob.FormattedAddress(), chainADenomOnChainB)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(transferAmount), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, transferAmount))

	aliceAddr, err := chainA.GetAddress(ctx, alice.KeyName())
	require.NoError(t, err)

	transfer = ibc.WalletAmount{
		Address: string(aliceAddr),
		Denom:   ibcDenomTrace.IBCDenom(),
		Amount:  transferAmount,
	}

	_, err = chainB.SendIBCTransfer(ctx, abChan.Counterparty.ChannelID, bob.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	aliceBal, err = chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, initBalance.Equal(aliceBal), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))

	bobBal, err = chainB.GetBalance(ctx, bob.FormattedAddress(), chainADenomOnChainB)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(math.ZeroInt()), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, math.ZeroInt()))

	//transfer = ibc.WalletAmount{
	//	Address: bob.FormattedAddress(),
	//	Denom:   chainA.Config().Denom,
	//	Amount:  transferAmount,
	//}
	//
	//h, err = chainB.Height(ctx)
	//require.NoError(t, err)
	//
	//_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, alice.KeyName(), transfer, ibc.TransferOptions{
	//	Timeout: &ibc.IBCTimeout{
	//		NanoSeconds: uint64((time.Duration(3) * time.Second).Nanoseconds()),
	//		Height:      h + 5,
	//	},
	//	Memo: "",
	//})
	//require.NoError(t, err)
	//
	//err = testutil.WaitForBlocks(ctx, 7, chainA)
	//require.NoError(t, err)
	//
	//aliceBal, err = chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	//require.NoError(t, err)
	//require.True(t, initBalance.Equal(aliceBal), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))
	//
	//bobBal, err = chainB.GetBalance(ctx, bob.FormattedAddress(), chainADenomOnChainB)
	//require.NoError(t, err)
	//require.True(t, bobBal.Equal(math.ZeroInt()), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, math.ZeroInt()))
}
