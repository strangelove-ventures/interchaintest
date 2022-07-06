package ibctest_test

import (
	"context"
	"fmt"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPacketForwardMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	home := ibctest.TempDir(t)
	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", ChainName: "gaia-fork", Version: "bugfix-replace_default_transfer_with_router_module"},
		{Name: "osmosis", ChainName: "osmosis", Version: "v7.3.0"},
		{Name: "juno", ChainName: "juno", Version: "v6.0.0"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia, osmosis, juno := chains[0], chains[1], chains[2]

	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network, home,
	)

	const pathOsmoHub = "osmohub"
	const pathJunoHub = "junohub"

	ic := ibctest.NewInterchain().
		AddChain(osmosis).
		AddChain(gaia).
		AddChain(juno).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  osmosis,
			Chain2:  gaia,
			Relayer: r,
			Path:    pathOsmoHub,
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  juno,
			Relayer: r,
			Path:    pathJunoHub,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, osmosis, gaia, juno)

	channels, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)

	// Start the relayer on both paths
	err = r.StartRelayer(ctx, eRep, pathOsmoHub)
	require.NoError(t, err)

	err = r.StartRelayer(ctx, eRep, pathJunoHub)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
			for _, c := range chains {
				if err = c.Cleanup(ctx); err != nil {
					t.Logf("an error occured while stopping chain %s: %s", c.Config().ChainID, err)
				}
			}
		},
	)

	// Get original account balances
	osmosisUser := users[0]
	gaiaUser := users[1]
	junoUser := users[2]

	osmosisBalOG, err := osmosis.GetBalance(ctx, osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix), osmosis.Config().Denom)
	require.NoError(t, err)

	// Send packet from Osmosis->Hub->Juno
	// receiver format: {intermediate_refund_address}|{foward_port}/{forward_channel}:{final_destination_address}
	const transferAmount int64 = 100000
	receiver := fmt.Sprintf("%s|%s/%s:%s", gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), channels[1].PortID, channels[1].ChannelID, junoUser.Bech32Address(juno.Config().Bech32Prefix))
	transfer := ibc.WalletAmount{
		Address: receiver,
		Denom:   osmosis.Config().Denom,
		Amount:  transferAmount,
	}

	_, err = osmosis.SendIBCTransfer(ctx, channels[0].ChannelID, osmosisUser.KeyName, transfer, nil)
	require.NoError(t, err)

	// Wait for transfer to be relayed
	err = test.WaitForBlocks(ctx, 10, gaia)
	require.NoError(t, err)

	// Check that the funds sent are gone from the acc on osmosis
	osmosisBal, err := osmosis.GetBalance(ctx, osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix), osmosis.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, osmosisBalOG-transferAmount, osmosisBal)

	// Compose the prefixed denoms and ibc denom for asserting balances
	firstHopDenom := transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, osmosis.Config().Denom)
	secondHopDenom := transfertypes.GetPrefixedDenom(channels[1].Counterparty.PortID, channels[1].Counterparty.ChannelID, firstHopDenom)
	dstIbcDenom := transfertypes.ParseDenomTrace(secondHopDenom)

	// Check that the funds sent are present in the acc on juno
	junoBal, err := juno.GetBalance(ctx, junoUser.Bech32Address(juno.Config().Bech32Prefix), dstIbcDenom.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, transferAmount, junoBal)

	// Send packet back from Juno->Hub->Osmosis
	receiver = fmt.Sprintf("%s|%s/%s:%s", gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix))
	transfer = ibc.WalletAmount{
		Address: receiver,
		Denom:   dstIbcDenom.IBCDenom(),
		Amount:  transferAmount,
	}

	_, err = juno.SendIBCTransfer(ctx, channels[1].Counterparty.ChannelID, junoUser.KeyName, transfer, nil)
	require.NoError(t, err)

	// Wait for transfer to be relayed
	err = test.WaitForBlocks(ctx, 10, gaia)
	require.NoError(t, err)

	// Check that the funds sent are gone from the acc on juno
	junoBal, err = juno.GetBalance(ctx, junoUser.Bech32Address(juno.Config().Bech32Prefix), dstIbcDenom.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, int64(0), junoBal)

	// Check that the funds sent are present in the acc on osmosis
	osmosisBal, err = osmosis.GetBalance(ctx, osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix), osmosis.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, osmosisBalOG, osmosisBal)

	// Send a malformed packet with invalid receiver address from Osmosis->Hub->Juno
	// This should succeed in the first hop and fail to make the second hop; funds should end up in the intermediary account.
	receiver = fmt.Sprintf("%s|%s/%s:%s", gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), channels[1].PortID, channels[1].ChannelID, "xyz1t8eh66t2w5k67kwurmn5gqhtq6d2ja0vp7jmmq")
	transfer = ibc.WalletAmount{
		Address: receiver,
		Denom:   osmosis.Config().Denom,
		Amount:  transferAmount,
	}

	_, err = osmosis.SendIBCTransfer(ctx, channels[0].ChannelID, osmosisUser.KeyName, transfer, nil)
	require.NoError(t, err)

	// Wait for transfer to be relayed
	err = test.WaitForBlocks(ctx, 10, gaia)
	require.NoError(t, err)

	// Check that the funds sent are gone from the acc on osmosis
	osmosisBal, err = osmosis.GetBalance(ctx, osmosisUser.Bech32Address(osmosis.Config().Bech32Prefix), osmosis.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, osmosisBalOG-transferAmount, osmosisBal)

	// Check that the funds sent ended up in the acc on gaia
	intermediaryIBCDenom := transfertypes.ParseDenomTrace(firstHopDenom)
	gaiaBal, err := gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), intermediaryIBCDenom.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, transferAmount, gaiaBal)
}
