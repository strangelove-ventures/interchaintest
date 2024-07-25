package thorchain_test

import (
	"context"
	"testing"
	
	"cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestThorchain(t *testing.T) {
	numThorchainValidators := 1
	numThorchainFullNodes  := 0
	numGaiaVals := 1
	numGaiaFn := 0

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	chainSpecs := []*interchaintest.ChainSpec{
		ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes),
		{
			Name: "gaia",
			Version: "v18.1.0",
			NumValidators: &numGaiaVals,
			NumFullNodes: &numGaiaFn,
		},
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*thorchain.Thorchain)
	gaia := chains[1].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain).
		AddChain(gaia)

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

	err = chain.StartAllValSidecars(ctx)
	require.NoError(t, err, "failed starting validator sidecars")

	fundAmount := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount, chain)
	thorchainUser := users[0]
	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "thorchain failed to make blocks")

	// Check balances are correct
	thorchainUserAmount, err := chain.GetBalance(ctx, thorchainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, thorchainUserAmount.Equal(fundAmount), "Initial thorchain user amount not expected")
	
	val0 := chain.GetNode()
	faucetAddr, err := val0.AccountKeyBech32(ctx, "faucet")
	require.NoError(t, err)
	faucetAmount, err := chain.GetBalance(ctx, faucetAddr, chain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, InitialFaucetAmount.Sub(fundAmount).Sub(StaticGas), faucetAmount)
	
	err = testutil.WaitForBlocks(ctx, 300, chain)
	require.NoError(t, err, "thorchain failed to make blocks")
}


