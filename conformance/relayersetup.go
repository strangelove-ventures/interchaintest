package conformance

import (
	"context"
	"fmt"
	"testing"

	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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

		eRep := rep.RelayerExecReporter(t)
		req.NoError(r.CreateConnections(ctx, eRep, pathName))

		// Assert against the singly created connections individually.
		conns0, err := r.GetConnections(ctx, eRep, c0.Config().ChainID)
		req.NoError(err)

		req.Len(conns0, 1)
		conn0 := conns0[0]
		req.NotEmpty(conn0.ID)
		req.NotEmpty(conn0.ClientID)
		req.Equal(conn0.State, conntypes.OPEN.String())

		conns1, err := r.GetConnections(ctx, eRep, c1.Config().ChainID)
		req.NoError(err)

		req.Len(conns1, 1)
		conn1 := conns1[0]
		req.NotEmpty(conn1.ID)
		req.NotEmpty(conn1.ClientID)
		req.Equal(conn1.State, conntypes.OPEN.String())

		// Now validate counterparties.
		req.Equal(conn0.Counterparty.ClientId, conn1.ClientID)
		req.Equal(conn0.Counterparty.ConnectionId, conn1.ID)
		req.Equal(conn1.Counterparty.ClientId, conn0.ClientID)
		req.Equal(conn1.Counterparty.ConnectionId, conn0.ID)
	})
	if t.Failed() {
		return
	}

	t.Run("create channel", func(t *testing.T) {
		rep.TrackTest(t)
		req := require.New(rep.TestifyT(t))

		eRep := rep.RelayerExecReporter(t)
		req.NoError(r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{
			SourcePortName: "transfer",
			DestPortName:   "transfer",
			Order:          "unordered",
			Version:        "ics20-1",
		}))

		// Now validate that the channels correctly report as created.
		// GetChannels takes around two seconds with rly,
		// so getting the channels concurrently is a measurable speedup.
		eg, egCtx := errgroup.WithContext(ctx)
		var channels0, channels1 []ibc.ChannelOutput
		eg.Go(func() error {
			var err error
			channels0, err = r.GetChannels(egCtx, eRep, c0.Config().ChainID)
			return err
		})
		eg.Go(func() error {
			var err error
			channels1, err = r.GetChannels(egCtx, eRep, c1.Config().ChainID)
			return err
		})
		req.NoError(eg.Wait(), "failure retrieving channels")

		req.Len(channels0, 1)
		ch0 := channels0[0]

		req.Len(channels1, 1)
		ch1 := channels1[0]

		// Piecemeal assertions against each channel.
		// Not asserting against ConnectionHops or ChannelID.
		req.Equal(ch0.State, "STATE_OPEN")
		req.Equal(ch0.Ordering, "ORDER_UNORDERED")
		req.Equal(ch0.Counterparty, ibc.ChannelCounterparty{PortID: "transfer", ChannelID: ch1.ChannelID})
		req.Equal(ch0.Version, "ics20-1")
		req.Equal(ch0.PortID, "transfer")

		req.Equal(ch1.State, "STATE_OPEN")
		req.Equal(ch1.Ordering, "ORDER_UNORDERED")
		req.Equal(ch1.Counterparty, ibc.ChannelCounterparty{PortID: "transfer", ChannelID: ch0.ChannelID})
		req.Equal(ch1.Version, "ics20-1")
		req.Equal(ch1.PortID, "transfer")
	})
}
