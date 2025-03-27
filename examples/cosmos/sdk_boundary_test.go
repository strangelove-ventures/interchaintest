package cosmos_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/conformance"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type boundarySpecs struct {
	name           string
	chainSpecs     []*interchaintest.ChainSpec
	relayerVersion string
}

func TestSDKBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	tests := []boundarySpecs{
		{
			name: "sdk 45 <-> 50",
			chainSpecs: []*interchaintest.ChainSpec{
				{
					Name: "gaia", ChainName: "gaia", Version: "v7.0.3", // sdk 0.45.6
					NumValidators: &numValsOne, NumFullNodes: &numFullNodesZero,
				},
				{
					Name: "ibc-go-simd", ChainName: "simd-50", Version: "v8.5.1", // sdk v0.50.10
					NumValidators: &numValsOne, NumFullNodes: &numFullNodesZero,
				},
			},
			relayerVersion: rly.DefaultContainerVersion,
		},
		{
			name: "sdk 47 <-> 50",
			chainSpecs: []*interchaintest.ChainSpec{
				{
					Name: "ibc-go-simd", ChainName: "simd-47", Version: "v7.2.0", // sdk 0.47.3
					NumValidators: &numValsOne, NumFullNodes: &numFullNodesZero,
				},
				{
					Name: "ibc-go-simd", ChainName: "simd-50", Version: "v8.5.1", // sdk v0.50.10
					NumValidators: &numValsOne, NumFullNodes: &numFullNodesZero,
				},
			},
			relayerVersion: rly.DefaultContainerVersion,
		},
	}

	for _, tt := range tests {
		tt := tt
		testname := tt.name
		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			chains := interchaintest.CreateChainsWithChainSpecs(t, tt.chainSpecs)

			client, network := interchaintest.DockerSetup(t)

			chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

			const (
				path        = "ibc-path"
				relayerName = "relayer"
			)

			// Get a relayer instance
			rf := interchaintest.NewBuiltinRelayerFactory(
				ibc.CosmosRly,
				zaptest.NewLogger(t),
				relayer.CustomDockerImage(
					rly.DefaultContainerImage,
					tt.relayerVersion,
					rly.RlyDefaultUIDGID,
				),
			)

			r := rf.Build(t, client, network)

			ic := interchaintest.NewInterchain().
				AddChain(chain).
				AddChain(counterpartyChain).
				AddRelayer(r, relayerName).
				AddLink(interchaintest.InterchainLink{
					Chain1:  chain,
					Chain2:  counterpartyChain,
					Relayer: r,
					Path:    path,
				})

			ctx := context.Background()

			rep := testreporter.NewNopReporter()

			require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
				TestName:  t.Name(),
				Client:    client,
				NetworkID: network,
				// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
				SkipPathCreation: false,
			}))
			t.Cleanup(func() {
				_ = ic.Close()
			})

			// test IBC conformance
			conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)
		})
	}
}
