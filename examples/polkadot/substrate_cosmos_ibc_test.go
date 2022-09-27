package polkadot_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/relayer"
	"github.com/strangelove-ventures/ibctest/v5/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

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

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:    "composable",
			Version: "polkadot:v0.9.19,composable:centauri",
			ChainConfig: ibc.ChainConfig{
				ChainID: "rococo-local",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			ChainName: "gaia",
			Version:   "v7.0.3",
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	composable, gaia := chains[0], chains[1]

	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
		// These two fields are used to pass in a custom Docker image built locally
		//relayer.ImagePull(false),
		relayer.CustomDockerImage("ghcr.io/composablefi/relayer", "sub-create-client", "100:1000"),
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

	//r.LinkPath()
	//
	//composable.SendIBCTransfer()
	//
	//r.StartRelayer()
	//t.Cleanup(func() {
	//	err = r.StopRelayer(ctx, eRep)
	//	if err != nil {
	//		panic(err)
	//	}
	//})

	// Make assertions to determine if the token transfer was successful
}
