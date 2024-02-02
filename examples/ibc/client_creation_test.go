package ibc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	numVals      = 1
	numFullNodes = 0
)

type relayerImp struct {
	name           string
	relayer        ibc.RelayerImplementation
	relayerVersion string
}

func TestCreatClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	var tests = []relayerImp{
		{
			name:           "Cosmos Relayer",
			relayer:        ibc.CosmosRly,
			relayerVersion: "v2.5.0",
		},
		//TODO: Get hermes working and uncomment:

		// {
		// 	name:           "Hermes",
		// 	relayer:        ibc.Hermes,
		// 	relayerVersion: "v1.7.1",
		// },
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
				tt.relayer,
				zaptest.NewLogger(t),
				relayer.CustomDockerImage(
					rly.DefaultContainerImage,
					tt.relayerVersion,
					rly.RlyDefaultUidGid,
				),
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

			result := r.Exec(ctx, eRep, []string{"rly", "config", "show", "--home", "/home/relayer"}, []string{})

			fmt.Println("RESULT: ", string(result.Stdout))

			//TODO: need a way to get chain name from relayer config!!
			require.NoError(t,
				r.CreateClient(ctx, eRep,
					chain.Config().Name,             // Does not work
					counterpartyChain.Config().Name, // Does not work
					pathName, ibc.CreateClientOptions{},
				),
			)

			clientInfo, err := r.GetClients(ctx, eRep, chain.Config().ChainID)
			require.NoError(t, err)

			// TODO: ensure client exists
			fmt.Println(clientInfo)

		})

	}

}
