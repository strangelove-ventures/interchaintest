package ibctest_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInterchain_DuplicateChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	home := t.TempDir()
	pool, network := ibctest.DockerSetup(t)

	cf := ibctest.NewBuiltinChainFactory([]ibctest.BuiltinChainFactoryEntry{
		// Two otherwise identical chains that only differ by ChainID.
		{Name: "gaia", Version: "v7.0.1", ChainID: "cosmoshub-0", NumValidators: 2, NumFullNodes: 1},
		{Name: "gaia", Version: "v7.0.1", ChainID: "cosmoshub-1", NumValidators: 2, NumFullNodes: 1},
	}, zaptest.NewLogger(t))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia0, gaia1 := chains[0], chains[1]

	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, pool, network, home,
	)

	ic := ibctest.NewInterchain().
		AddChain(gaia0).
		AddChain(gaia1).
		AddRelayer(r, "r").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia0,
			Chain2:  gaia1,
			Relayer: r,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Pool:      pool,
		NetworkID: network,

		SkipPathCreation: true,
	}))
}
