package ibctest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This test is meant to be used as a basic ibctest tutorial.
// Code snippets are broken down in ./docs/upAndRunning.md
func TestLearn(t *testing.T) {
	ctx := context.Background()

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "v7.0.3"},
		{Name: "osmosis", Version: "v11.0.1"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer Factory
	client, network := ibctest.DockerSetup(t)
	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	// Prep Interchain
	const ibcPath = "gaia-osmo-demo"
	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	// Log location
	wd, err := os.Getwd()
	require.NoError(t, err)
	logFolder := filepath.Join(wd, "ibcTest_logs")
	os.MkdirAll(logFolder, os.ModePerm)
	require.NoError(t, err)
	f, err := os.Create(filepath.Join(logFolder, fmt.Sprintf("%d.json", time.Now().Unix())))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false},
	),
	)

	// Create and Fund User Wallets
	fundAmount := 1_000
	users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia, osmosis)
	gaiaUser := users[0]
	osmosisUser := users[1]

	gaiaUserBal, err := gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaia.Config().Denom)
	require.NoError(t, err)
	t.Log("INITIAL BALANCE!!!!", gaiaUserBal)

	// Get Channel ID
	gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	gaiaChannelID := gaiaChannelInfo[0].ChannelID

	osmoChannelInfo, err := r.GetChannels(ctx, eRep, osmosis.Config().ChainID)
	require.NoError(t, err)
	osmoChannelID := osmoChannelInfo[0].ChannelID

	// Send Transaction
	t.Run("send ibc transaction", func(t *testing.T) {
		rep.TrackTest(t)
		t.Log("osmosisUser.Bech32Address(gaia.Config().Bech32Prefix)!!: ", osmosisUser.Bech32Address(gaia.Config().Bech32Prefix))
		t.Log("osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix)!!: ", osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix))
		tx, err := gaia.SendIBCTransfer(ctx, gaiaChannelID, gaiaUser.KeyName, ibc.WalletAmount{
			Address: osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix),
			Denom:   gaia.Config().Denom,
			Amount:  100,
		},
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, tx.Validate())
	})

	// Relay Packet
	t.Run("relay packet", func(t *testing.T) {
		rep.TrackTest(t)
		require.NoError(t, r.FlushPackets(ctx, eRep, ibcPath, gaiaChannelID))
	})

	// Relay Acknowledgement
	t.Run("relay acknowledgement", func(t *testing.T) {
		rep.TrackTest(t)
		require.NoError(t, r.FlushAcknowledgements(ctx, eRep, ibcPath, osmoChannelID))
	})

	gaiaUserBal, err = gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaia.Config().Denom)
	require.NoError(t, err)
	t.Log("FINAL BALANCE!!!", gaiaUserBal)

}

// go test -timeout 300s -v -run ^TestTemplate$ github.com/strangelove-ventures/ibctest/examples
