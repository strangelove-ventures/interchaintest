package thorchain_test

import (
	"context"
	"testing"
	
	"cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestThorchain(t *testing.T) {
	numValidators := 4
	numFullNodes  := 0

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), ThorchainDefaultChainSpec(t.Name(), numValidators, numFullNodes))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*thorchain.Thorchain)

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

	fundAmount := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount, chain)
	thorchainUser := users[0]
	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "thorchain failed to make blocks")

	// Check balances are correct
	thorchainUserAmount, err := chain.GetBalance(ctx, thorchainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, thorchainUserAmount.Equal(fundAmount), "Initial thorchain user amount not expected")
}


