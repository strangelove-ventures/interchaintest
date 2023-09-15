package cosmos_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestICTestMiscellaneous(t *testing.T) {
	CosmosChainTestMiscellaneous(t, "juno", "v16.0.0")
}

func CosmosChainTestMiscellaneous(t *testing.T, name, version string) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      name,
			ChainName: name,
			Version:   version,
			ChainConfig: ibc.ChainConfig{
				Denom: "ujuno",
			},
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	BuildDependencies(ctx, t, chain)

}

func BuildDependencies(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	deps := chain.Validators[0].GetBuildInformation(ctx)

	sdkVer := "v0.47.3"

	require.Equal(t, deps.Name, "juno")
	require.Equal(t, deps.ServerName, "junod")
	require.Equal(t, deps.Version, "v16.0.0")
	require.Equal(t, deps.CosmosSdkVersion, sdkVer)
	require.Equal(t, deps.Commit, "054796f6173a9f15d012b656e255f94a4ec1d2cd")
	require.Equal(t, deps.BuildTags, "netgo muslc,")

	for _, dep := range deps.BuildDeps {
		dep := dep

		// Verify specific examples
		if dep.Parent == "github.com/cosmos/cosmos-sdk" {
			require.Equal(t, dep.Version, sdkVer)
			require.Equal(t, dep.IsReplacement, false)
		} else if dep.Parent == "github.com/99designs/keyring" {
			require.Equal(t, dep.Version, "v1.2.2")
			require.Equal(t, dep.IsReplacement, true)
			require.Equal(t, dep.Replacement, "github.com/cosmos/keyring")
			require.Equal(t, dep.ReplacementVersion, "v1.2.0")

		}

		// Verify all replacement logic
		if dep.IsReplacement {
			require.GreaterOrEqual(t, len(dep.ReplacementVersion), 6, "ReplacementVersion: %s must be >=6 length (ex: vA.B.C)", dep.ReplacementVersion)
			require.Greater(t, len(dep.Replacement), 0, "Replacement: %s must be >0 length.", dep.Replacement)
		} else {
			require.Equal(t, len(dep.Replacement), 0, "Replacement: %s is not 0.", dep.Replacement)
			require.Equal(t, len(dep.ReplacementVersion), 0, "ReplacementVersion: %s is not 0.", dep.ReplacementVersion)
		}
	}
}
