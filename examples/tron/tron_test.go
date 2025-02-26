package tron

import (
	"context"
	"cosmossdk.io/math"
	"fmt"
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

	fmt.Println(client)
	fmt.Println(network)

	factory := interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t), []*interchaintest.ChainSpec{
			{ChainConfig: tron.DefaultChainConfig("trontest")},
		},
	)

	chains, err := factory.Chains(t.Name())
	require.NoError(t, err)

	tronChain := chains[0].(*tron.Chain)

	fmt.Println(tronChain)

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

	time.Sleep(time.Second * 60)

	amount := math.NewInt(1_000_000_000)

	users := interchaintest.GetAndFundTestUsers(t, ctx, "user1", amount, tronChain)
	user1 := users[0]

	balance1, err := tronChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	require.Equal(t, amount, balance1)
}
