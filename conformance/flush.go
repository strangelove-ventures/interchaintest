package conformance

import (
	"context"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
)

func TestRelayerFlushing(t *testing.T, ctx context.Context, cf interchaintest.ChainFactory, rf interchaintest.RelayerFactory, rep *testreporter.Reporter) {
	rep.TrackTest(t)

	// FlushPackets will be exercised in a subtest,
	// but check that capability first in case we can avoid setup.
	requireCapabilities(t, rep, rf, relayer.Flush)

	client, network := interchaintest.DockerSetup(t)

	req := require.New(rep.TestifyT(t))
	chains, err := cf.Chains(t.Name())
	req.NoError(err, "failed to get chains")

	if len(chains) != 2 {
		panic(fmt.Errorf("expected 2 chains, got %d", len(chains)))
	}

	c0, c1 := chains[0], chains[1]

	r := rf.Build(t, client, network)

	const pathName = "p"
	ic := interchaintest.NewInterchain().
		AddChain(c0).
		AddChain(c1).
		AddRelayer(r, "r").
		AddLink(interchaintest.InterchainLink{
			Chain1:  c0,
			Chain2:  c1,
			Relayer: r,

			Path:              pathName,
			CreateChannelOpts: ibc.DefaultChannelOpts(),
		})

	eRep := rep.RelayerExecReporter(t)

	req.NoError(ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	}))
	defer ic.Close()

	// Get faucet address on destination chain for ibc transfer.
	c1FaucetAddrBytes, err := c1.GetAddress(ctx, interchaintest.FaucetAccountKeyName)
	req.NoError(err)
	c1FaucetAddr, err := types.Bech32ifyAddressBytes(c1.Config().Bech32Prefix, c1FaucetAddrBytes)
	req.NoError(err)

	channels, err := r.GetChannels(ctx, eRep, c0.Config().ChainID)
	req.NoError(err)
	req.Len(channels, 1)

	c0ChannelID := channels[0].ChannelID

	beforeTransferHeight, err := c0.Height(ctx)
	req.NoError(err)

	const txAmount = 112233 // Arbitrary amount that is easy to find in logs.
	tx, err := c0.SendIBCTransfer(ctx, c0ChannelID, interchaintest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: c1FaucetAddr,
		Denom:   c0.Config().Denom,
		Amount:  txAmount,
	}, ibc.TransferOptions{})
	req.NoError(err)
	req.NoError(tx.Validate())

	t.Run("flush", func(t *testing.T) {
		rep.TrackTest(t)

		eRep := rep.RelayerExecReporter(t)

		req := require.New(rep.TestifyT(t))

		// Should trigger MsgRecvPacket.
		req.NoError(r.Flush(ctx, eRep, pathName, c0ChannelID))

		afterFlushHeight, err := c0.Height(ctx)
		req.NoError(err)

		_, err = testutil.PollForAck(ctx, c0, beforeTransferHeight, afterFlushHeight+5, tx.Packet)
		req.NoError(err)
	})
}
