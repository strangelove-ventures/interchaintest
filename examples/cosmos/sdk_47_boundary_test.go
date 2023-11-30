package cosmos_test

import (
	"context"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/conformance"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// this tests ibc between a sdk 45 and an sdk 47 chain
// it tests relaying compatibility with both the Go Relayer and Hermes

type relayerSpec struct {
	name           string
	implementation ibc.RelayerImplementation
	version        string
}

func TestSDK47Boundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	var relayersToTest = []relayerSpec{
		{
			name:           "Go-Relayer",
			implementation: ibc.CosmosRly,
			version:        "v2.4.2",
		},
		{
			name:           "Hermes",
			implementation: ibc.Hermes,
			version:        "v1.7.3",
		},
	}

	// test both Go Relayer and Hermes
	for _, relayerSpec := range relayersToTest {
		relayerSpec := relayerSpec
		testname := relayerSpec.name
		t.Run(testname, func(t *testing.T) {
			t.Parallel()

			cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
				{
					Name:      "gaia",
					ChainName: "gaia",
					Version:   "v7.0.3", // sdk 45.6
				},
				{
					Name:      "ibc-go-simd",
					ChainName: "ibc-go-simd",
					Version:   "v7.3.1", // sdk 47.5
				},
			})

			chains, err := cf.Chains(t.Name())
			require.NoError(t, err)

			client, network := interchaintest.DockerSetup(t)

			chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

			const (
				path        = "ibc-paths"
				relayerName = "relayer"
			)

			// Get a relayer instance
			rf := interchaintest.NewBuiltinRelayerFactory(
				relayerSpec.implementation,
				zaptest.NewLogger(t),
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
				TestName:         t.Name(),
				Client:           client,
				NetworkID:        network,
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
