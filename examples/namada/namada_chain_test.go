package namada_test

import (
	"context"
	"fmt"
	stdmath "math"
	"strconv"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	namadachain "github.com/strangelove-ventures/interchaintest/v8/chain/namada"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNamadaNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 1
	fn := 0

	coinDecimals := namadachain.NamTokenDenom
	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "gaia", Version: "v19.2.0", ChainConfig: ibc.ChainConfig{
			GasPrices: "1uatom",
		}},
		{
			Name:    "namada",
			Version: "v0.44.1",
			ChainConfig: ibc.ChainConfig{
				ChainID:      "namada-test",
				Denom:        namadachain.NamAddress,
				Gas:          "250000",
				CoinDecimals: &coinDecimals,
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get namada chain")
	chain := chains[0]
	namada := chains[1].(*namadachain.NamadaChain)

	// Relayer Factory
	r := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t),
		relayer.CustomDockerImage(
			"ghcr.io/heliaxdev/hermes",
			"v1.10.4-namada-beta17-rc2@sha256:a95ede57f63ebb4c70aa4ca0bfb7871a5d43cd76d17b1ad62f5d31a9465d65af",
			"2000:2000",
		)).
		Build(t, client, network)

	// Prep Interchain
	const ibcPath = "namada-ibc-test"
	ic := interchaintest.NewInterchain().
		AddChain(chain).
		AddChain(namada).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chain,
			Chain2:  namada,
			Relayer: r,
			Path:    ibcPath,
		})

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Build interchain
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

	initBalance := math.NewInt(1_000_000)

	gasSpent, _ := strconv.ParseInt(namada.Config().Gas, 10, 64)
	namadaGasSpent := math.NewInt(gasSpent)
	tokenDenom := math.NewInt(int64(stdmath.Pow(10, float64(*namada.Config().CoinDecimals))))
	namadaInitBalance := initBalance.Mul(tokenDenom)

	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chain, namada)
	chainUser := users[0]
	namadaUser := users[1]

	chainUserBalInitial, err := chain.GetBalance(ctx, chainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, chainUserBalInitial.Equal(initBalance))

	namadaUserBalInitial, err := namada.GetBalance(ctx, namadaUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalInitial.Equal(namadaInitBalance))

	// Get Channel ID
	chainChannelInfo, err := r.GetChannels(ctx, eRep, chain.Config().ChainID)
	require.NoError(t, err)
	chainChannelID := chainChannelInfo[0].ChannelID
	namadaChannelInfo, err := r.GetChannels(ctx, eRep, namada.Config().ChainID)
	require.NoError(t, err)
	namadaChannelID := namadaChannelInfo[0].ChannelID

	// 1. Send Transaction from the chain to Namada
	amountToSend := math.NewInt(1)
	dstAddress := namadaUser.FormattedAddress()
	transfer := ibc.WalletAmount{
		Address: dstAddress,
		Denom:   chain.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err := chain.SendIBCTransfer(ctx, chainChannelID, chainUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay packets
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, chainChannelID))

	// test source wallet has decreased funds
	expectedBal := chainUserBalInitial.Sub(amountToSend).Sub(math.NewInt(tx.GasSpent))
	chainUserBalAfter1, err := chain.GetBalance(ctx, chainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, chainUserBalAfter1.Equal(expectedBal))

	// Test destination wallet has increased funds
	dstIbcTrace := transfertypes.GetPrefixedDenom("transfer", namadaChannelID, chain.Config().Denom)
	namadaUserIbcBalAfter1, err := namada.GetBalance(ctx, namadaUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaUserIbcBalAfter1.Equal(amountToSend))

	// 2. Send Transaction from Namada to the chain
	amountToSend = math.NewInt(1)
	dstAddress = chainUser.FormattedAddress()
	transfer = ibc.WalletAmount{
		Address: dstAddress,
		Denom:   namada.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err = namada.SendIBCTransfer(ctx, namadaChannelID, namadaUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay packets
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, namadaChannelID))

	// test source wallet has decreased funds
	expectedBal = namadaUserBalInitial.Sub(amountToSend.Mul(tokenDenom)).Sub(namadaGasSpent)
	namadaUserBalAfter2, err := namada.GetBalance(ctx, namadaUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalAfter2.Equal(expectedBal))

	// test destination wallet has increased funds
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", chainChannelID, namada.Config().Denom))
	dstIbcDenom := srcDenomTrace.IBCDenom()
	chainUserIbcBalAfter2, err := chain.GetBalance(ctx, chainUser.FormattedAddress(), dstIbcDenom)
	require.NoError(t, err)
	require.True(t, chainUserIbcBalAfter2.Equal(amountToSend.Mul(tokenDenom)))

	// 3. Shielding transfer (chain -> Namada's shielded account) test
	// generate a shielded account
	users = interchaintest.GetAndFundTestUsers(t, ctx, "shielded", initBalance, namada)
	namadaShieldedUser := users[0].(*namadachain.NamadaWallet)
	namadaShieldedUserBalInitial, err := namada.GetBalance(ctx, namadaShieldedUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaShieldedUserBalInitial.Equal(namadaInitBalance))

	amountToSend = math.NewInt(1)
	destAddress, err := namada.GetAddress(ctx, namadaShieldedUser.PaymentAddressKeyName())
	require.NoError(t, err)
	transfer = ibc.WalletAmount{
		Address: string(destAddress),
		Denom:   chain.Config().Denom,
		Amount:  amountToSend,
	}
	// generate the IBC shielding transfer from the destination Namada
	shieldedTransfer, err := namada.GenIbcShieldingTransfer(ctx, namadaChannelID, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	// replace the destination address with the MASP address
	// because it has been already set in the IBC shielding transfer
	transfer.Address = namadachain.MaspAddress
	tx, err = chain.SendIBCTransfer(ctx, chainChannelID, chainUser.KeyName(), transfer, ibc.TransferOptions{
		Memo: shieldedTransfer,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay packets
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, chainChannelID))

	// test source wallet has decreased funds
	expectedBal = chainUserBalAfter1.Sub(amountToSend).Sub(math.NewInt(tx.GasSpent))
	chainUserBalAfter3, err := chain.GetBalance(ctx, chainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, chainUserBalAfter3.Equal(expectedBal))

	// test destination wallet has increased funds
	dstIbcTrace = transfertypes.GetPrefixedDenom("transfer", namadaChannelID, chain.Config().Denom)
	namadaShieldedUserIbcBalAfter3, err := namada.GetBalance(ctx, namadaShieldedUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUserIbcBalAfter3.Equal(amountToSend))

	// 4. Shielded transfer (Shielded account 1 -> Shielded account 2) on Namada
	// generate another shielded account
	users = interchaintest.GetAndFundTestUsers(t, ctx, "shielded", initBalance, namada)
	namadaShieldedUser2 := users[0].(*namadachain.NamadaWallet)
	namadaShieldedUser2BalInitial, err := namada.GetBalance(ctx, namadaShieldedUser2.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaShieldedUser2BalInitial.Equal(namadaInitBalance))

	amountToSend = math.NewInt(1)
	transfer = ibc.WalletAmount{
		Address: namadaShieldedUser2.FormattedAddress(),
		Denom:   dstIbcTrace,
		Amount:  amountToSend,
	}
	err = namada.ShieldedTransfer(ctx, namadaShieldedUser.KeyName(), transfer)
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// test source wallet has decreased funds
	expectedBal = namadaShieldedUserIbcBalAfter3.Sub(amountToSend)
	namadaShieldedUserBalAfter4, err := namada.GetBalance(ctx, namadaShieldedUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUserBalAfter4.Equal(expectedBal))

	// test destination wallet has increased funds
	namadaShieldedUser2IbcBalAfter4, err := namada.GetBalance(ctx, namadaShieldedUser2.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUser2IbcBalAfter4.Equal(amountToSend))

	// 5. Unshielding transfer (Namada's shielded account 2 -> chain) test
	amountToSend = math.NewInt(1)
	dstAddress = chainUser.FormattedAddress()
	transfer = ibc.WalletAmount{
		Address: dstAddress,
		Denom:   dstIbcTrace,
		Amount:  amountToSend,
	}
	tx, err = namada.SendIBCTransfer(ctx, namadaChannelID, namadaShieldedUser2.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay packets
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, namadaChannelID))

	// test source wallet has decreased funds
	expectedBal = namadaShieldedUser2IbcBalAfter4.Sub(amountToSend)
	namadaShieldedUser2BalAfter5, err := namada.GetBalance(ctx, namadaShieldedUser2.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUser2BalAfter5.Equal(expectedBal))

	// test destination wallet has increased funds
	expectedBal = chainUserBalAfter3.Add(amountToSend)
	chainUserIbcBalAfter4, err := chain.GetBalance(ctx, chainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, chainUserIbcBalAfter4.Equal(expectedBal))
}
