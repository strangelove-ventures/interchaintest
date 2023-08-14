package ibc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This test is meant to be used as a basic interchaintest tutorial.
// Code snippets are broken down in ./docs/upAndRunning.md
func TestLearn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	// Chain Factory
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "gaia", Version: "v7.0.0", ChainConfig: ibc.ChainConfig{
			GasPrices: "0.0uatom",
		}},
		{Name: "osmosis", Version: "v11.0.0"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer Factory
	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	// Prep Interchain
	const ibcPath = "gaia-osmo-demo"
	ic := interchaintest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false},
	),
	)

	// Create and Fund User Wallets
	fundAmount := math.NewInt(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount.Int64(), gaia, osmosis)
	gaiaUser := users[0]
	osmosisUser := users[1]

	gaiaUserBalInitial, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalInitial.Equal(fundAmount))

	// Get Channel ID
	gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannelID := gaiaChannelInfo[0].ChannelID

	osmoChannelInfo, err := r.GetChannels(ctx, eRep, osmosis.Config().ChainID)
	require.NoError(t, err)
	osmoChannelID := osmoChannelInfo[0].ChannelID

	// Send Transaction
	amountToSend := math.NewInt(1_000_000)
	dstAddress := osmosisUser.FormattedAddress()
	transfer := ibc.WalletAmount{
		Address: dstAddress,
		Denom:   gaia.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err := gaia.SendIBCTransfer(ctx, gaiaChannelID, gaiaUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	// relay MsgRecvPacket to osmosis, then MsgAcknowledgement back to gaia
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, gaiaChannelID))

	// test source wallet has decreased funds
	expectedBal := gaiaUserBalInitial.Sub(amountToSend)
	gaiaUserBalNew, err := gaia.GetBalance(ctx, gaiaUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaUserBalNew.Equal(expectedBal))

	// Trace IBC Denom
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", osmoChannelID, gaia.Config().Denom))
	dstIbcDenom := srcDenomTrace.IBCDenom()

	// Test destination wallet has increased funds
	osmosUserBalNew, err := osmosis.GetBalance(ctx, osmosisUser.FormattedAddress(), dstIbcDenom)
	require.NoError(t, err)
	require.True(t, osmosUserBalNew.Equal(amountToSend))
}
