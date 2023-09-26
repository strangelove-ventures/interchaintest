package cosmos_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestICTestMiscellaneous(t *testing.T) {
	CosmosChainTestMiscellaneous(t, "juno", "v16.0.0")
}

const (
	initialBalance = 100_000_000
)

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

	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", int64(initialBalance), chain, chain)

	TokenFactory(ctx, t, chain, users)
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

func TokenFactory(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	user := users[0]
	user2 := users[1]

	subDenom := "ictest"
	tfDenom, err := cosmos.TokenFactoryCreateDenom(chain, ctx, user, subDenom, 2500000)
	require.NoError(t, err)
	require.Equal(t, tfDenom, "factory/"+user.FormattedAddress()+"/"+subDenom)

	// modify metadata
	stdout, err := cosmos.TokenFactoryMetadata(chain, ctx, user.KeyName(), tfDenom, "SYMBOL", "description here", 6)
	t.Log(stdout, err)
	require.NoError(t, err)

	// verify metadata
	md, err := chain.QueryBankMetadata(ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, md.Metadata.Description, "description here")
	require.Equal(t, md.Metadata.Symbol, "SYMBOL")
	require.Equal(t, md.Metadata.DenomUnits[1].Exponent, 6)

	// mint tokens
	_, err = cosmos.TokenFactoryMintDenom(chain, ctx, user.KeyName(), tfDenom, 1)
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 1)

	// mint-to
	_, err = cosmos.TokenFactoryMintDenomTo(chain, ctx, user.KeyName(), tfDenom, 3, user2.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user2, tfDenom, 3)

	// force transfer 1 from user2 (3) to user1 (1)
	_, err = cosmos.TokenFactoryForceTransferDenom(chain, ctx, user.KeyName(), tfDenom, 1, user2.FormattedAddress(), user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 2)
	validateBalance(ctx, t, chain, user2, tfDenom, 2)

	// burn token from account
	_, err = cosmos.TokenFactoryBurnDenomFrom(chain, ctx, user.KeyName(), tfDenom, 1, user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 1)

	prevAdmin, err := cosmos.TokenFactoryGetAdmin(chain, ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, prevAdmin.AuthorityMetadata.Admin, user.FormattedAddress())

	// change admin, then we will burn the token-from
	_, err = cosmos.TokenFactoryChangeAdmin(chain, ctx, user.KeyName(), tfDenom, user2.FormattedAddress())
	require.NoError(t, err)

	// validate new admin is set
	tfAdmin, err := cosmos.TokenFactoryGetAdmin(chain, ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, tfAdmin.AuthorityMetadata.Admin, user2.FormattedAddress())

	// burn
	_, err = cosmos.TokenFactoryBurnDenomFrom(chain, ctx, user2.KeyName(), tfDenom, 1, user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 0)

}

func validateBalance(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, user ibc.Wallet, tfDenom string, expected int64) {
	balance, err := chain.GetBalance(ctx, user.FormattedAddress(), tfDenom)
	require.NoError(t, err)
	require.Equal(t, balance, math.NewInt(expected))
}
