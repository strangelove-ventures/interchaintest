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

// Need to set the Namada directory before running this test
// `export ENV_NAMADA_REPO=/path/to/namada`
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
			Version: "main",
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
	gaia := chains[0]
	namada := chains[1].(*namadachain.NamadaChain)

	// Relayer Factory
	r := interchaintest.NewBuiltinRelayerFactory(ibc.Hermes, zaptest.NewLogger(t),
		relayer.CustomDockerImage(
			"ghcr.io/heliaxdev/hermes",
			"v1.10.3-namada-beta16-rc@sha256:9ebecd51fb9aecefee840b264a4eb2eccd58f64bfdf2f0a5fe2b6613c947b422",
			"2000:2000",
		)).
		Build(t, client, network)

	// Prep Interchain
	const ibcPath = "gaia-namada-demo"
	ic := interchaintest.NewInterchain().
		AddChain(gaia).
		AddChain(namada).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
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

	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, gaia, namada)
	gaiaUser := users[0]
	namadaUser := users[1]

	gaiaUserBalInitial, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalInitial.Equal(initBalance))

	namadaUserBalInitial, err := namada.GetBalance(ctx, namadaUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalInitial.Equal(namadaInitBalance))

	// Get Channel ID
	gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannelID := gaiaChannelInfo[0].ChannelID
	namadaChannelInfo, err := r.GetChannels(ctx, eRep, namada.Config().ChainID)
	require.NoError(t, err)
	namadaChannelID := namadaChannelInfo[0].ChannelID

	// 1. Send Transaction from Gaia to Namada
	amountToSend := math.NewInt(1)
	dstAddress := namadaUser.FormattedAddress()
	transfer := ibc.WalletAmount{
		Address: dstAddress,
		Denom:   gaia.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err := gaia.SendIBCTransfer(ctx, gaiaChannelID, gaiaUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay MsgRecvPacket to namada, then MsgAcknowledgement back to gaia
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, gaiaChannelID))

	// test source wallet has decreased funds
	expectedBal := gaiaUserBalInitial.Sub(amountToSend).Sub(math.NewInt(tx.GasSpent))
	gaiaUserBalAfter1, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalAfter1.Equal(expectedBal))

	// Test destination wallet has increased funds
	dstIbcTrace := transfertypes.GetPrefixedDenom("transfer", namadaChannelID, gaia.Config().Denom)
	namadaUserIbcBalAfter1, err := namada.GetBalance(ctx, namadaUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaUserIbcBalAfter1.Equal(amountToSend))

	// 2. Send Transaction from Namada to Gaia
	amountToSend = math.NewInt(1)
	dstAddress = gaiaUser.FormattedAddress()
	transfer = ibc.WalletAmount{
		Address: dstAddress,
		Denom:   namada.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err = namada.SendIBCTransfer(ctx, namadaChannelID, namadaUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay MsgRecvPacket to namada, then MsgAcknowledgement back to gaia
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, namadaChannelID))

	// test source wallet has decreased funds
	expectedBal = namadaUserBalInitial.Sub(amountToSend.Mul(tokenDenom)).Sub(namadaGasSpent)
	namadaUserBalAfter2, err := namada.GetBalance(ctx, namadaUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalAfter2.Equal(expectedBal))

	// test destination wallet has increased funds
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", gaiaChannelID, namada.Config().Denom))
	dstIbcDenom := srcDenomTrace.IBCDenom()
	gaiaUserIbcBalAfter2, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), dstIbcDenom)
	require.NoError(t, err)
	require.True(t, gaiaUserIbcBalAfter2.Equal(amountToSend.Mul(tokenDenom)))

	// 3. Shielding transfer (Gaia -> Namada's shielded account) test
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
		Denom:   gaia.Config().Denom,
		Amount:  amountToSend,
	}
	// generate the IBC shielding transfer from the destination Namada
	shieldedTransfer, err := namada.GenIbcShieldingTransfer(ctx, namadaChannelID, transfer, ibc.TransferOptions{})
	require.NoError(t, err)

	// replace the destination address with the MASP address because the destination payment address has been already set in the IBC shielding transfer
	transfer.Address = namadachain.MaspAddress
	tx, err = gaia.SendIBCTransfer(ctx, gaiaChannelID, gaiaUser.KeyName(), transfer, ibc.TransferOptions{
		Memo: shieldedTransfer,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay MsgRecvPacket to namada, then MsgAcknowledgement back to gaia
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, gaiaChannelID))

	// test source wallet has decreased funds
	expectedBal = gaiaUserBalAfter1.Sub(amountToSend).Sub(math.NewInt(tx.GasSpent))
	gaiaUserBalAfter3, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalAfter3.Equal(expectedBal))

	// test destination wallet has increased funds
	dstIbcTrace = transfertypes.GetPrefixedDenom("transfer", namadaChannelID, gaia.Config().Denom)
	namadaShieldedUserIbcBalAfter3, err := namada.GetBalance(ctx, namadaShieldedUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUserIbcBalAfter3.Equal(amountToSend))

	// 4. Unshielding transfer (Namada's shielded account -> Gaia) test
	amountToSend = math.NewInt(1)
	dstAddress = gaiaUser.FormattedAddress()
	transfer = ibc.WalletAmount{
		Address: dstAddress,
		Denom:   dstIbcTrace,
		Amount:  amountToSend,
	}
	tx, err = namada.SendIBCTransfer(ctx, namadaChannelID, namadaShieldedUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay MsgRecvPacket to namada, then MsgAcknowledgement back to gaia
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, namadaChannelID))

	// test source wallet has decreased funds
	expectedBal = namadaShieldedUserIbcBalAfter3.Sub(amountToSend)
	namadaShieldedUserBalAfter4, err := namada.GetBalance(ctx, namadaShieldedUser.KeyName(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaShieldedUserBalAfter4.Equal(expectedBal))

	// test destination wallet has increased funds
	expectedBal = gaiaUserBalAfter3.Add(amountToSend)
	gaiaUserIbcBalAfter4, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserIbcBalAfter4.Equal(expectedBal))
}
