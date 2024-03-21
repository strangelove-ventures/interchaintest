package ibc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This tests Cosmos Interchain Security, spinning up a provider and a single consumer chain.
func TestICS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

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

			// Chain Factory
			cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
				{Name: "ics-provider", Version: "v3.1.0", ChainConfig: ibc.ChainConfig{GasAdjustment: 1.5}},
				{Name: "ics-consumer", Version: "v3.1.0"},
			})

			chains, err := cf.Chains(t.Name())
			require.NoError(t, err)
			provider, consumer := chains[0], chains[1]

			// Relayer Factory
			client, network := interchaintest.DockerSetup(t)

			r := interchaintest.NewBuiltinRelayerFactory(
				tt.relayerImp,
				zaptest.NewLogger(t),
			).Build(t, client, network)

			// Prep Interchain
			const ibcPath = "ics-path"
			ic := interchaintest.NewInterchain().
				AddChain(provider).
				AddChain(consumer).
				AddRelayer(r, "relayer").
				AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
					Provider: provider,
					Consumer: consumer,
					Relayer:  r,
					Path:     ibcPath,
				})

			// Log location
			f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
			require.NoError(t, err)
			// Reporter/logs
			rep := testreporter.NewReporter(f)
			eRep := rep.RelayerExecReporter(t)

			// Build interchain
			err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
				TestName:          t.Name(),
				Client:            client,
				NetworkID:         network,
				BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

				SkipPathCreation: false,
			})
			require.NoError(t, err, "failed to build interchain")

			err = testutil.WaitForBlocks(ctx, 10, provider, consumer)
			require.NoError(t, err, "failed to wait for blocks")
		})
	}
}
