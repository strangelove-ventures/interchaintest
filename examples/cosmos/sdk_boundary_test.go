package cosmos_test

import (
	"context"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/conformance"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
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

	var tests = []boundarySpecs{
		{
			name: "sdk 45 <-> 50",
			chainSpecs: []*interchaintest.ChainSpec{
				{
					Name: "gaia", ChainName: "gaia", Version: "v7.0.3", //sdk 0.45.6
				},
				{
					Name: "ibc-go-simd", ChainName: "simd-50", Version: "feat-upgrade-sdk-v0.50", //sdk 0.50 alpha
				},
			},
			relayerVersion: "colin-event-fix",
		},
		{
			name: "sdk 47 <-> 50",
			chainSpecs: []*interchaintest.ChainSpec{
				{
					Name: "ibc-go-simd", ChainName: "simd-47", Version: "v7.2.0", //sdk 0.47.3
				},
				{
					Name: "ibc-go-simd", ChainName: "simd-50", Version: "feat-upgrade-sdk-v0.50", //sdk 0.50 alpha
				},
			},
			relayerVersion: "colin-event-fix",
		},
	}

	for _, tt := range tests {
		tt := tt
		testname := tt.name
		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			cf := interchaintest.NewBuiltinChainFactory(
				zaptest.NewLogger(t),
				tt.chainSpecs,
			)

			chains, err := cf.Chains(t.Name())
			require.NoError(t, err)

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
					rly.RlyDefaultUidGid,
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
				TestName:          t.Name(),
				Client:            client,
				NetworkID:         network,
				BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
				SkipPathCreation:  false,
			}))
			t.Cleanup(func() {
				_ = ic.Close()
			})

			// test IBC conformance
			conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)

		})
	}

}
