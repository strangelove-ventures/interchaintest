package avalanche_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/strangelove-ventures/interchaintest/v8"
	subnetevm "github.com/strangelove-ventures/interchaintest/v8/examples/avalanche/subnet-evm"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

func TestAvalancheCosmos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 5
	nf := 0

	subnetChainID := "99999"

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "avalanche",
			Version: "v1.10.18-ibc",
			ChainConfig: ibc.ChainConfig{
				ChainID: "1337",
				Images: []ibc.DockerImage{
					{
						Repository: "avalanchego",
						Version:    "v1.10.18-ibc",
						UidGid:     "1025:1025",
					},
				},
				AvalancheSubnets: []ibc.AvalancheSubnetConfig{
					{
						Name:                "subnetevm",
						ChainID:             subnetChainID,
						Genesis:             subnetevm.Genesis,
						SubnetClientFactory: subnetevm.NewSubnetEvmClient,
					},
				},
				CoinType: "60",
			},
			NumFullNodes:  &nf,
			NumValidators: &nv,
		},
		{
			ChainName: "simd", // Set chain name so that a suffix with a "dash" is not appended (required for hyperspace)
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "simd",
				ChainID: "simd",
				Images: []ibc.DockerImage{
					{
						Repository: "simd",
						Version:    "avalanche-light-client", // Set your locally built version
						UidGid:     "1025:1025",
					},
				},
				Bin:            "simd",
				Bech32Prefix:   "cosmos",
				Denom:          "stake",
				GasPrices:      "0.00stake",
				GasAdjustment:  1.3,
				TrustingPeriod: "504h",
				CoinType:       "118",
				//NoHostMount:         true,
				//ConfigFileOverrides: configFileOverrides,
				//ModifyGenesis:       modifyGenesisShortProposals(votingPeriod, maxDepositPeriod),
			},
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get avalanche chain")
	require.Len(t, chains, 2)

	avalancheChain := chains[0]
	cosmosChain := chains[1]

	ctx := context.Background()

	// Get a relayer instance
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		// These two fields are used to pass in a custom Docker image built locally
		relayer.ImagePull(false),
		relayer.CustomDockerImage("relayer", "avalanche", "1000:1000"), // Set your locally built version
	).Build(t, client, network)

	pathName := "ibc-path"

	ic := interchaintest.NewInterchain().
		AddChain(avalancheChain).
		AddChain(cosmosChain).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  avalancheChain,
			Chain2:  cosmosChain,
			Relayer: r,
			Path:    pathName,
		})

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = r.GeneratePath(ctx, eRep, cosmosChain.Config().ChainID, subnetChainID, pathName)
	require.NoError(t, err)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName, ibc.DefaultClientOpts())
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, avalancheChain) // these 1 block waits seem to be needed to reduce flakiness
	require.NoError(t, err)

	// Create a new connection
	err = r.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, avalancheChain)
	require.NoError(t, err)

	// Create a new channel & get channels from each chain
	err = r.CreateChannel(ctx, eRep, pathName, ibc.DefaultChannelOpts())
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, avalancheChain)
	require.NoError(t, err)

	// Start relayer
	r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = r.StopRelayer(ctx, eRep)
		if err != nil {
			panic(err)
		}
	})

}
