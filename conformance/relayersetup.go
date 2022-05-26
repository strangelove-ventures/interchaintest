package conformance

import (
	"context"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
)

// TestRelayerSetup contains a series of subtests that configure a relayer step-by-step.
func TestRelayerSetup(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory, rep *testreporter.Reporter) {
	rep.TrackTest(t)

	home := t.TempDir()
	pool, network := ibctest.DockerSetup(t)

	req := require.New(rep.TestifyT(t))
	chains, err := cf.Chains(t.Name())
	req.NoError(err, "failed to get chains")

	if len(chains) != 2 {
		panic(fmt.Errorf("expected 2 chains, got %d", len(chains)))
	}

	c0, c1 := chains[0], chains[1]

	r := rf.Build(t, pool, network, home)

	const pathName = "p"
	ic := ibctest.NewInterchain().
		AddChain(c0).
		AddChain(c1).
		AddRelayer(r, "r").
		AddLink(ibctest.InterchainLink{
			// We are adding a link here so that the interchain object creates appropriate relayer wallets,
			// but we call ic.Build with SkipPathCreation=true, so the link won't be created.
			Chain1:  c0,
			Chain2:  c1,
			Relayer: r,

			Path: pathName,
		})

	ctx := context.Background()
	eRep := rep.RelayerExecReporter(t)

	req.NoError(ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Pool:      pool,
		NetworkID: network,

		// Create relayer keys and wallets but don't create links,
		// since that is what we are about to test.
		SkipPathCreation: true,
	}))

	// Now handle each creation step as a subtest in sequence.
	// After each subtest, check t.Failed, which will be true if a subtest failed,
	// to conditionally stop execution before the following subtest.

	t.Run("generate path", func(t *testing.T) {
		rep.TrackTest(t)
		req := require.New(rep.TestifyT(t))

		req.NoError(r.GeneratePath(ctx, rep.RelayerExecReporter(t), c0.Config().ChainID, c1.Config().ChainID, pathName))
	})
	if t.Failed() {
		return
	}

	t.Run("create clients", func(t *testing.T) {
		rep.TrackTest(t)
		req := require.New(rep.TestifyT(t))

		req.NoError(r.CreateClients(ctx, rep.RelayerExecReporter(t), pathName))
	})
	if t.Failed() {
		return
	}

	// The client isn't created immediately -- wait for two blocks to ensure the clients are ready.
	req.NoError(test.WaitForBlocks(ctx, 2, c0, c1))

	t.Run("create connections", func(t *testing.T) {
		rep.TrackTest(t)
		req := require.New(rep.TestifyT(t))

		req.NoError(r.CreateConnections(ctx, rep.RelayerExecReporter(t), pathName))
	})
	if t.Failed() {
		return
	}

	t.Run("create channels", func(t *testing.T) {
		rep.TrackTest(t)

		rep.TrackSkip(t, "ibc.Relayer does not yet have a CreateChannels method")
	})
}
