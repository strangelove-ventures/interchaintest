package cosmos_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestUpdateLightClients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	// Chains
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "gaia", Version: gaiaVersion},
		{Name: "osmosis", Version: osmosisVersion},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// Relayer
	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    "client-test-path",
		})

	// Build interchain
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	require.NoError(t, r.StartRelayer(ctx, eRep))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, eRep)
	})

	// Create and Fund User Wallets
	fundAmount := int64(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount, gaia, osmosis)
	gaiaUser, osmoUser := users[0], users[1]

	// Get Channel ID
	gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
	require.NoError(t, err)
	chanID := gaiaChannelInfo[0].ChannelID

	height, err := osmosis.Height(ctx)
	require.NoError(t, err)

	amountToSend := int64(553255) // Unique amount to make log searching easier.
	dstAddress := osmoUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(osmosis.Config().Bech32Prefix)
	transfer := ibc.WalletAmount{
		Address: dstAddress,
		Denom:   gaia.Config().Denom,
		Amount:  math.NewInt(amountToSend),
	}
	tx, err := gaia.SendIBCTransfer(ctx, chanID, gaiaUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())

	chain := osmosis.(*cosmos.CosmosChain)
	reg := chain.Config().EncodingConfig.InterfaceRegistry
	msg, err := cosmos.PollForMessage[*clienttypes.MsgUpdateClient](ctx, chain, reg, height, height+10, nil)
	require.NoError(t, err)

	require.Equal(t, "07-tendermint-0", msg.ClientId)
	require.NotEmpty(t, msg.Signer)
	// TODO: Assert header information
}
