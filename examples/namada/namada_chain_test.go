package namada_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/namada"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNamadaNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 1
	fn := 1

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "namada",
			Version: "main",
			ChainConfig: ibc.ChainConfig{
				ChainID: "namada-test",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get namada chain")
	require.Len(t, chains, 1)
	chain := chains[0].(*namada.NamadaChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	rep := testreporter.NewNopReporter()

	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))

	t.Cleanup(func() {
		err := ic.Close()
		if err != nil {
			panic(err)
		}
	})

	faucetBalInitial, err := chain.GetBalance(ctx, "faucet", "nam")
	require.NoError(t, err)
	require.True(t, faucetBalInitial.Equal(math.NewInt(100000000)))

	initBalance := math.NewInt(1_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance, chain)
	require.Equal(t, 1, len(users))

	userBalInitial, err := chain.GetBalance(ctx, "user", "nam")
	require.NoError(t, err)
	require.True(t, userBalInitial.Equal(initBalance))
}
