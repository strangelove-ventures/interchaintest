package penumbra

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPenumbraToPenumbraIBC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 2
	fn := 0

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "penumbra",
			Version: "v0.60.0,v0.34.24",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbra-1",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
		{
			Name:    "penumbra",
			Version: "v0.60.0,v0.34.24",
			ChainConfig: ibc.ChainConfig{
				ChainID: "penumbra-2",
			},
			NumValidators: &nv,
			NumFullNodes:  &fn,
		},
	},
	).Chains(t.Name())
	require.NoError(t, err, "failed to get penumbra chains")
	require.Len(t, chains, 2)

	chainA := chains[0].(*penumbra.PenumbraChain)
	chainB := chains[1].(*penumbra.PenumbraChain)

	i := ibc.DockerImage{
		Repository: "ghcr.io/cosmos/relayer",
		Version:    "main",
		UidGid:     "1025:1025",
	}
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.DockerImage(&i),
	).Build(t, client, network)

	const pathName = "ab"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:            chainA,
			Chain2:            chainB,
			Relayer:           r,
			Path:              pathName,
			CreateClientOpts:  ibc.CreateClientOptions{},
			CreateChannelOpts: ibc.CreateChannelOptions{},
		})

	ctx := context.Background()
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		err := ic.Close()
		if err != nil {
			panic(err)
		}
	})

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				panic(fmt.Errorf("an error occured while stopping the relayer: %s", err))
			}
		},
	)

	// Fund users and check init balances
	initBalance := math.NewInt(1_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance.Int64(), chainA)
	require.Equal(t, 1, len(users))

	alice := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	aliceBal, err := chainA.GetBalance(ctx, alice.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, aliceBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", aliceBal, initBalance))

	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", initBalance.Int64(), chainA)
	require.Equal(t, 1, len(users))

	bob := users[0]

	err = testutil.WaitForBlocks(ctx, 5, chainA)
	require.NoError(t, err)

	bobBal, err := chainA.GetBalance(ctx, bob.KeyName(), chainA.Config().Denom)
	require.NoError(t, err)
	require.True(t, bobBal.Equal(initBalance), fmt.Sprintf("incorrect balance, got (%s) expected (%s)", bobBal, initBalance))

	//bobAddr, err := chainA.GetAddress(ctx, bob.KeyName())
	//require.NoError(t, err)
}
