package polkadot_test

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/internal/configutil"
	"github.com/strangelove-ventures/ibctest/v5/relayer"
	"github.com/strangelove-ventures/ibctest/v5/relayer/rly"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
)

func TestKeyGen(t *testing.T) {
	key := rly.GenKey()
	fmt.Println(key)
}

// TestSubstrateToCosmosIBC simulates a Parachain to Cosmos IBC integration by spinning up an IBC enabled
// Parachain along with an IBC enabled Cosmos chain, attempting to create an IBC path between the two chains,
// and initiating an ics20 token transfer between the two.
func TestSubstrateToCosmosIBC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nv := 5 // Number of validators
	nf := 3 // Number of full nodes

	configFileOverrides := make(configutil.Toml)

	appTomlOverrides := make(configutil.Toml)
	configTomlOverrides := make(configutil.Toml)

	apiOverrides := make(configutil.Toml)
	apiOverrides["rpc-max-body-bytes"] = 13500000
	appTomlOverrides["api"] = apiOverrides

	rpcOverrides := make(configutil.Toml)
	rpcOverrides["max_body_bytes"] = 13500000
	rpcOverrides["max_header_bytes"] = 14000000
	configTomlOverrides["rpc"] = rpcOverrides

	//mempoolOverrides := make(testutil.Toml)
	//mempoolOverrides["max_tx_bytes"] = 6000000
	//configTomlOverrides["mempool"] = mempoolOverrides

	configFileOverrides["config/app.toml"] = appTomlOverrides
	configFileOverrides["config/config.toml"] = configTomlOverrides

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name: "rococo-local",
			//Name:    "composable",
			ChainName: "rococo-local",
			Version:   "polkadot-node:local,parachain-node:local",
			//Version:   "seunlanlege/centauri-polkadot:v0.9.27,seunlanlege/centauri-parachain:v0.9.27",
			ChainConfig: ibc.ChainConfig{
				Name:         "rococo-local",
				ChainID:      "rococo-local",
				Bech32Prefix: "composable",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			Name:    "ibcgo",
			Version: "latest",
			ChainConfig: ibc.ChainConfig{
				Bech32Prefix:        "cosmos",
				ConfigFileOverrides: configFileOverrides,
			},
			/*
				ChainName: "gaia",
				Version:   "v7.0.3",
			*/
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	composable, gaia := chains[0], chains[1]

	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.Hyperspace,
		zaptest.NewLogger(t),
		//relayer.StartupFlags("-b", "100"),
		// These two fields are used to pass in a custom Docker image built locally
		relayer.ImagePull(false),
		//relayer.CustomDockerImage("ghcr.io/composablefi/relayer", "sub-create-client", "100:1000"),
		//relayer.CustomDockerImage("go-relayer", "local", "100:1000"),
		relayer.CustomDockerImage("hyperspace", "latest", "501:20"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "composable-gaia"
	const relayerName = "relayer"

	ic := ibctest.NewInterchain().
		AddChain(composable).
		AddChain(gaia).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  composable,
			Chain2:  gaia,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	fundAmount := int64(10_000_000)
	chainCfg := gaia.Config()
	key := rly.GenKey()
	err = gaia.SendFunds(ctx, "faucet", ibc.WalletAmount{
		Address: types.MustBech32ifyAddressBytes(chainCfg.Bech32Prefix, key.Address),
		//Address: user.Bech32Address(chainCfg.Bech32Prefix),
		Amount: fundAmount,
		Denom:  chainCfg.Denom,
	})
	require.NoError(t, err)
	//users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia)
	//gaiaUser := users[0]
	//gaiaUserBalInitial, err := gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaia.Config().Denom)
	//require.NoError(t, err)
	//require.Equal(t, fundAmount, gaiaUserBalInitial)

	time.Sleep(3000000 * time.Second)

	// Here we need to create a connection, channel and the client via CLI. After it's done,
	// we can uncomment the code below and start working on it
	/*
		// If necessary you can wait for x number of blocks to pass before taking some action
		//blocksToWait := 10
		//err = test.WaitForBlocks(ctx, blocksToWait, composable)
		//require.NoError(t, err)

		// Generate a new IBC path between the chains
		// This is like running `rly paths new`
		err = r.GeneratePath(ctx, eRep, composable.Config().ChainID, gaia.Config().ChainID, pathName)
		require.NoError(t, err)

		// Attempt to create the light clients for both chains on the counterparty chain
		err = r.CreateClients(ctx, rep.RelayerExecReporter(t), pathName, ibc.DefaultClientOpts())
		require.NoError(t, err)

		// Once client, connection, and handshake logic is implemented for the Substrate provider
		// we can link the path, start the relayer and attempt to send a token transfer via IBC.

		t.Cleanup(func() {
			fmt.Println("Cleaning up in 30 seconds...")
			time.Sleep(30 * time.Second)
			_ = ic.Close()
		})

		err = r.StartRelayer(ctx, eRep, pathName)
		require.NoError(t, err)

		t.Cleanup(func() {
			fmt.Println("Cleaning up in 30 seconds...")
			time.Sleep(30 * time.Second)
			err = r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(err)
			}
		})

		const userFunds = int64(10_000_000_000)
		users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, gaia, composable)
		gaiaUser, composableUser := users[0], users[1]

		gaiaUserBalInitial, err := gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaia.Config().Denom)
		require.NoError(t, err)
		require.Equal(t, userFunds, gaiaUserBalInitial)

		// Get Channel ID
		gaiaChannelInfo, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)
		require.NoError(t, err)
		gaiaChannelID := gaiaChannelInfo[0].ChannelID

		osmoChannelInfo, err := r.GetChannels(ctx, eRep, composable.Config().ChainID)
		require.NoError(t, err)
		osmoChannelID := osmoChannelInfo[0].ChannelID

		// Send Transaction
		amountToSend := int64(1_000_000)
		dstAddress := composableUser.Bech32Address(composable.Config().Bech32Prefix)
		tx, err := gaia.SendIBCTransfer(ctx, gaiaChannelID, gaiaUser.KeyName, ibc.WalletAmount{
			Address: dstAddress,
			Denom:   gaia.Config().Denom,
			Amount:  amountToSend,
		},
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		// relay packets and acknoledgments
		require.NoError(t, r.FlushPackets(ctx, eRep, pathName, osmoChannelID))
		require.NoError(t, r.FlushAcknowledgements(ctx, eRep, pathName, gaiaChannelID))

		// test source wallet has decreased funds
		expectedBal := gaiaUserBalInitial - amountToSend
		gaiaUserBalNew, err := gaia.GetBalance(ctx, gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaia.Config().Denom)
		require.NoError(t, err)
		require.Equal(t, expectedBal, gaiaUserBalNew)

		// Trace IBC Denom
		srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", gaiaChannelID, gaia.Config().Denom))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		// Test destination wallet has increased funds
		osmosUserBalNew, err := composable.GetBalance(ctx, composableUser.Bech32Address(composable.Config().Bech32Prefix), dstIbcDenom)
		require.NoError(t, err)
		require.Equal(t, amountToSend, osmosUserBalNew)

		//gaiaChannels, err := r.GetChannels(ctx, eRep, gaia.Config().ChainID)

		//r.LinkPath(ctx, eRep, pathName, nil, nil)

	*/
	/*
		// osmosis = composable
		// gaia = juno = gaia

		// composable -> gaia
		composableUser, gaiaUser := users[0], users[1]

		composableBalOG, err := composable.GetBalance(ctx, composableUser.Bech32Address(composable.Config().Bech32Prefix), composable.Config().Denom)
		require.NoError(t, err)
		require.Equal(t, userFunds, composableBalOG)

		// Send packet from Osmosis->Hub->Juno
		// receiver format: {intermediate_refund_address}|{foward_port}/{forward_channel}:{final_destination_address}

		const transferAmount int64 = 100000
		gaiaJunoChan := gaiaChannels[0].Counterparty
		receiver := fmt.Sprintf("%s|%s/%s:%s", gaiaUser.Bech32Address(gaia.Config().Bech32Prefix), gaiaJunoChan.PortID, gaiaJunoChan.ChannelID, junoUser.Bech32Address(juno.Config().Bech32Prefix))
		transfer := ibc.WalletAmount{
			Address: receiver,
			Denom:   composable.Config().Denom,
			Amount:  transferAmount,
		}

		composableChannels, err := r.GetChannels(ctx, eRep, composable.Config().ChainID)
		composableGaiaChan := composableChannels[0]

		_, err = composable.SendIBCTransfer(ctx, composableGaiaChan.ChannelID, composableUser.KeyName, transfer, nil)
		require.NoError(t, err)

		// Wait for transfer to be relayed
		err = test.WaitForBlocks(ctx, 10, gaia)
		require.NoError(t, err)

		// Check that the funds sent are gone from the acc on composable
		composableBal, err := composable.GetBalance(ctx, composableUser.Bech32Address(composable.Config().Bech32Prefix), composable.Config().Denom)
		require.NoError(t, err)
		require.Equal(t, composableBalOG-transferAmount, composableBal)

	*/
	//composable.SendIBCTransfer()
	//_, err = composable.SendIBCTransfer(ctx, junoGaiaChan.ChannelID, junoUser.KeyName, transfer, nil)
	//require.NoError(t, err)

	// Make assertions to determine if the token transfer was successful
}
