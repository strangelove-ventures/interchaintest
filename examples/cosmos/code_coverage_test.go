package cosmos_test

import (
	"context"
	"os"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func TestCodeCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	var (
		ctx                = context.Background()
		ExternalGoCoverDir = "/tmp/interchaintest-app-coverage"
		Denom              = "umfx"
	)

	cfgA := ibc.ChainConfig{
		Type:    "cosmos",
		Name:    "manifest",
		ChainID: "manifest-2",
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/liftedinit/manifest-ledger",
				Version:    "v0.0.1-alpha.10",
				UIDGID:     "1025:1025",
			},
		},
		Bin:            "manifestd",
		Bech32Prefix:   "manifest",
		Denom:          Denom,
		GasPrices:      "0" + Denom,
		GasAdjustment:  1.3,
		TrustingPeriod: "508h",
		NoHostMount:    false,
	}

	cfgA.WithCodeCoverage()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel)), []*interchaintest.ChainSpec{
		{
			Name:          "manifest",
			Version:       cfgA.Images[0].Version,
			ChainName:     cfgA.Name,
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodesZero,
			ChainConfig:   cfgA,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)

	client, network := interchaintest.DockerSetup(t)

	ic := interchaintest.NewInterchain().
		AddChain(chainA)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		dockerutil.CopyCoverageFromContainer(ctx, t, client, chainA.GetNode().ContainerID(), chainA.HomeDir(), ExternalGoCoverDir)

		files, err := os.ReadDir(ExternalGoCoverDir)
		require.NoError(t, err)
		require.NotEmpty(t, files)

		_ = ic.Close()
	})
}
