package polkadot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v9"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/relayer"
	"github.com/strangelove-ventures/interchaintest/v9/testreporter"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
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

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nv := 5 // Number of validators
	nf := 3 // Number of full nodes

	// Get both chains
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			//Name:    "composable",
			//Version: "seunlanlege/centauri-polkadot:v0.9.27,seunlanlege/centauri-parachain:v0.9.27",
			ChainConfig: ibc.ChainConfig{
				Type:    "polkadot",
				Name:    "composable",
				ChainID: "rococo-local",
				Images: []ibc.DockerImage{
					{
						Repository: "seunlanlege/centauri-polkadot",
						Version:    "v0.9.27",
						UidGid:     "1025:1025",
					},
					{
						Repository: "seunlanlege/centauri-parachain",
						Version:    "v0.9.27",
						UidGid:     "1025:1025",
					},
				},
				Bin:            "polkadot",
				Bech32Prefix:   "composable",
				Denom:          "uDOT",
				GasPrices:      "",
				GasAdjustment:  0,
				TrustingPeriod: "",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "ibc-go-simd",
				ChainID: "simd",
				Images: []ibc.DockerImage{
					{
						Repository: "ibc-go-simd",
						Version:    "feat-wasm-client",
						UidGid:     "1025:1025",
					},
				},
				Bin:            "simd",
				Bech32Prefix:   "cosmos",
				Denom:          "stake",
				GasPrices:      "0.00stake",
				GasAdjustment:  1.3,
				TrustingPeriod: "504h",
				//EncodingConfig: WasmClientEncoding(),
				NoHostMount: true,
				//ConfigFileOverrides: configFileOverrides,
			},
			/*
				ChainName: "gaia",
				Version:   "v7.0.3",
			*/
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	composable, simd := chains[0], chains[1]

	// Get a relayer instance
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
		// These two fields are used to pass in a custom Docker image built locally
		//relayer.ImagePull(false),
		relayer.CustomDockerImage("ghcr.io/composablefi/relayer", "sub-create-client", "100:1000"),
		//relayer.CustomDockerImage("go-relayer", "local", "100:1000"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "composable-simd"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(composable).
		AddChain(simd) //.
	//AddRelayer(r, relayerName).
	/*AddLink(interchaintest.InterchainLink{
		Chain1:  composable,
		Chain2:  simd,
		Relayer: r,
		Path:    pathName,
	})*/

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))

	// If necessary you can wait for x number of blocks to pass before taking some action
	//blocksToWait := 10
	//err = testutil.WaitForBlocks(ctx, blocksToWait, composable)
	//require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 2000, simd)
	require.NoError(t, err)
	// Generate a new IBC path between the chains
	// This is like running `rly paths new`
	err = r.GeneratePath(ctx, eRep, composable.Config().ChainID, simd.Config().ChainID, pathName)
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

	t.Cleanup(func() {
		fmt.Println("Cleaning up in 30 seconds...")
		time.Sleep(30 * time.Second)
		_ = ic.Close()
	})
}
