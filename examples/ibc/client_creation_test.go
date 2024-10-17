package ibc_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	numVals      = 1
	numFullNodes = 0
)

type relayerImp struct {
	name       string
	relayerImp ibc.RelayerImplementation
}

func TestCreatClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	tests := []relayerImp{
		{
			name:       "Cosmos Relayer",
			relayerImp: ibc.CosmosRly,
		},
		{
			name:       "Hermes",
			relayerImp: ibc.Hermes,
		},
	}

	for _, tt := range tests {
		tt := tt
		testname := tt.name
		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			chainSpec := []*interchaintest.ChainSpec{
				{
					Name:      "ibc-go-simd",
					ChainName: "chain1",
					Version:   "v8.0.0",

					NumValidators: &numVals,
					NumFullNodes:  &numFullNodes,
				},
				{
					Name:      "ibc-go-simd",
					ChainName: "chain2",
					Version:   "v8.0.0",

					NumValidators: &numVals,
					NumFullNodes:  &numFullNodes,
				},
			}

			chains := interchaintest.CreateChainsWithChainSpecs(t, chainSpec)

			client, network := interchaintest.DockerSetup(t)

			chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

			rf := interchaintest.NewBuiltinRelayerFactory(
				tt.relayerImp,
				zaptest.NewLogger(t),
			)

			r := rf.Build(t, client, network)

			pathName := "ibc-path"

			ic := interchaintest.NewInterchain().
				AddChain(chain).
				AddChain(counterpartyChain).
				AddRelayer(r, "relayer").
				AddLink(interchaintest.InterchainLink{
					Chain1:  chain,
					Chain2:  counterpartyChain,
					Relayer: r,
					Path:    pathName,
				})

			rep := testreporter.NewNopReporter()

			require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
				TestName:         t.Name(),
				Client:           client,
				NetworkID:        network,
				SkipPathCreation: true,
			}))
			t.Cleanup(func() {
				_ = ic.Close()
			})

			eRep := rep.RelayerExecReporter(t)

			// Get clients for each chain
			srcClientInfoBefore, err := r.GetClients(ctx, eRep, chain.Config().ChainID)
			require.NoError(t, err)
			destClientInfoBefore, err := r.GetClients(ctx, eRep, counterpartyChain.Config().ChainID)
			require.NoError(t, err)

			require.NoError(t,
				r.GeneratePath(ctx, eRep, chain.Config().ChainID, counterpartyChain.Config().ChainID, pathName))

			// create single client
			require.NoError(t,
				r.CreateClient(ctx, eRep,
					chain.Config().ChainID,
					counterpartyChain.Config().ChainID,
					pathName, ibc.CreateClientOptions{},
				),
			)

			srcClientInfoAfter, err := r.GetClients(ctx, eRep, chain.Config().ChainID)
			require.NoError(t, err)
			destClientInfoAfter, err := r.GetClients(ctx, eRep, counterpartyChain.Config().ChainID)
			require.NoError(t, err)

			// After creating the single client on the source chain, there should be one more client than before
			require.Equal(t, len(srcClientInfoBefore), (len(srcClientInfoAfter) - 1), "there is not exactly 1 more client on the source chain after running createClient")

			// createClients should only create a client on source, NOT destination chain
			require.Equal(t, len(destClientInfoBefore), len(destClientInfoAfter), "a client was created on the destination chain")
		})

	}
}
