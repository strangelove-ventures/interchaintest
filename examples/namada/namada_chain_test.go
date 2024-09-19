package namada_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
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

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "gaia", Version: "v19.2.0", ChainConfig: ibc.ChainConfig{
			GasPrices: "1uatom",
		}},
		{
			Name:    "namada",
			Version: "main",
			ChainConfig: ibc.ChainConfig{
				ChainID: "namada-test",
				Denom:   "tnam1qxgfw7myv4dh0qna4hq0xdg6lx77fzl7dcem8h7e",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get namada chain")
	gaia, namada := chains[0], chains[1]

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
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, gaia, namada)
	gaiaUser := users[0]
	namadaUser := users[1]

	gaiaUserBalInitial, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalInitial.Equal(initBalance))

	namadaUserBalInitial, err := namada.GetBalance(ctx, namadaUser.KeyName(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalInitial.Equal(initBalance))

	// Get Channel ID
	gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannelID := gaiaChannelInfo[0].ChannelID
	namadaChannelInfo, err := r.GetChannels(ctx, eRep, namada.Config().ChainID)
	require.NoError(t, err)
	namadaChannelID := namadaChannelInfo[0].ChannelID

	// Send Transaction from Gaia to Namada
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
	gaiaUserBalNew, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalNew.Equal(expectedBal))

	// Test destination wallet has increased funds
	dstIbcTrace := transfertypes.GetPrefixedDenom("transfer", namadaChannelID, gaia.Config().Denom)
	namadaUserBalNew, err := namada.GetBalance(ctx, namadaUser.FormattedAddress(), dstIbcTrace)
	require.NoError(t, err)
	require.True(t, namadaUserBalNew.Equal(amountToSend))

	// Send Transaction from Namada to Gaia
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
	expectedBal = namadaUserBalInitial.Sub(amountToSend).Sub(math.NewInt(tx.GasSpent))
	namadaUserBalNew, err = namada.GetBalance(ctx, namadaUser.FormattedAddress(), namada.Config().Denom)
	require.NoError(t, err)
	require.True(t, namadaUserBalNew.Equal(amountToSend))

	// Test destination wallet has increased funds
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", gaiaChannelID, namada.Config().Denom))
	dstIbcDenom := srcDenomTrace.IBCDenom()
	expectedBal = gaiaUserBalInitial.Add(amountToSend.Mul(math.NewInt(1000000)))
	gaiaUserBalNew, err = gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), dstIbcDenom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalNew.Equal(expectedBal))
}
