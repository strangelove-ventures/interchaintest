package tron

import (
	"context"
	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/tron"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
)

func TestTron(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	factory := interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t), []*interchaintest.ChainSpec{
			{ChainConfig: tron.DefaultChainConfig("trontest")},
		},
	)

	chains, err := factory.Chains(t.Name())
	require.NoError(t, err)

	tronChain := chains[0].(*tron.Chain)

	interchain := interchaintest.NewInterchain().
		AddChain(tronChain)

	require.NoError(t, interchain.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))

	t.Cleanup(func() {
		_ = interchain.Close()
	})

	block, err := tronChain.Height(ctx)
	require.NoError(t, err)
	require.Greater(t, block, int64(0))

	amount := math.NewInt(1_000_000_000)

	users := interchaintest.GetAndFundTestUsers(t, ctx, "user1", amount, tronChain)
	user1 := users[0]

	err = tronChain.WaitBlocks(ctx, 10, time.Second*40)
	require.NoError(t, err)

	balance1, err := tronChain.GetBalance(ctx, user1.FormattedAddress(), "TRX")
	require.NoError(t, err)
	require.Equal(t, amount, balance1)
}
