package blockdb_test

// This test is in a separate file, so it can be in the blockdb_test package,
// so it can import interchaintest without creating an import cycle.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestMessagesView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	const gaia0ChainID = "g0"
	const gaia1ChainID = "g1"
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: gaia0ChainID}},
		{Name: "gaia", Version: "v7.0.1", ChainConfig: ibc.ChainConfig{ChainID: gaia1ChainID}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia0, gaia1 := chains[0], chains[1]

	rf := interchaintest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t))
	r := rf.Build(t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(gaia0).
		AddChain(gaia1).
		AddRelayer(r, "r").
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia0,
			Chain2:  gaia1,
			Relayer: r,
		})

	dbDir := interchaintest.TempDir(t)
	dbPath := filepath.Join(dbDir, "blocks.db")

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true,

		BlockDatabaseFile: dbPath,
	}))

	// The database should exist on disk,
	// but no transactions should have happened yet.
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Copy the busy timeout from the migration.
	// The journal_mode pragma should be persisted on disk, so we should not need to set that here.
	_, err = db.Exec(`PRAGMA busy_timeout = 3000`)
	require.NoError(t, err)

	var count int
	row := db.QueryRow(`SELECT COUNT(*) FROM v_cosmos_messages`)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, count, 0)

	// Generate the path.
	// No transactions happen here.
	const pathName = "p"
	require.NoError(t, r.GeneratePath(ctx, eRep, gaia0ChainID, gaia1ChainID, pathName))

	t.Run("create clients", func(t *testing.T) {
		// Creating the clients will cause transactions.
		require.NoError(t, r.CreateClients(ctx, eRep, pathName, ibc.DefaultClientOpts()))

		// MsgCreateClient should match the opposite chain IDs.
		const qCreateClient = `SELECT
client_chain_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.client.v1.MsgCreateClient" AND chain_id = ?;`
		var clientChainID string
		require.NoError(t, db.QueryRow(qCreateClient, gaia0ChainID).Scan(&clientChainID))
		require.Equal(t, clientChainID, gaia1ChainID)

		require.NoError(t, db.QueryRow(qCreateClient, gaia1ChainID).Scan(&clientChainID))
		require.Equal(t, clientChainID, gaia0ChainID)
	})
	if t.Failed() {
		return
	}

	var gaia0ClientID, gaia0ConnID, gaia1ClientID, gaia1ConnID string
	t.Run("create connections", func(t *testing.T) {
		// The client isn't created immediately -- wait for two blocks to ensure the clients are ready.
		require.NoError(t, testutil.WaitForBlocks(ctx, 2, gaia0, gaia1))

		// Next, create the connections.
		require.NoError(t, r.CreateConnections(ctx, eRep, pathName))

		// Wait for another block before retrieving the connections and querying for them.
		require.NoError(t, testutil.WaitForBlocks(ctx, 1, gaia0, gaia1))

		conns, err := r.GetConnections(ctx, eRep, gaia0ChainID)
		require.NoError(t, err)

		// Collect the reported client IDs.
		gaia0ConnID = conns[0].ID
		gaia0ClientID = conns[0].ClientID
		gaia1ConnID = conns[0].Counterparty.ConnectionId
		gaia1ClientID = conns[0].Counterparty.ClientId

		// OpenInit happens on first chain.
		const qConnectionOpenInit = `SELECT
client_id, counterparty_client_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenInit" AND chain_id = ?
`
		var clientID, counterpartyClientID string
		require.NoError(t, db.QueryRow(qConnectionOpenInit, gaia0ChainID).Scan(&clientID, &counterpartyClientID))
		require.Equal(t, clientID, gaia0ClientID)
		require.Equal(t, counterpartyClientID, gaia1ClientID)

		// OpenTry happens on second chain.
		const qConnectionOpenTry = `SELECT
counterparty_client_id, counterparty_conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenTry" AND chain_id = ?
`
		var counterpartyConnID string
		require.NoError(t, db.QueryRow(qConnectionOpenTry, gaia1ChainID).Scan(&counterpartyClientID, &counterpartyConnID))
		require.Equal(t, counterpartyClientID, gaia0ClientID)
		require.Equal(t, counterpartyConnID, gaia0ConnID)

		// OpenAck happens on first chain again.
		const qConnectionOpenAck = `SELECT
conn_id, counterparty_conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenAck" AND chain_id = ?
`
		var connID string
		require.NoError(t, db.QueryRow(qConnectionOpenAck, gaia0ChainID).Scan(&connID, &counterpartyConnID))
		require.Equal(t, connID, gaia0ConnID)
		require.Equal(t, counterpartyConnID, gaia1ConnID)

		// OpenConfirm happens on second chain again.
		const qConnectionOpenConfirm = `SELECT
conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenConfirm" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qConnectionOpenConfirm, gaia1ChainID).Scan(&connID))
		require.Equal(t, connID, gaia0ConnID) // Not sure if this should be connection 0 or 1, as they are typically equal during this test.
	})
	if t.Failed() {
		return
	}

	const gaia0Port, gaia1Port = "transfer", "transfer" // Would be nice if these could differ.
	var gaia0ChannelID, gaia1ChannelID string
	t.Run("create channel", func(t *testing.T) {
		require.NoError(t, r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{
			SourcePortName: gaia0Port,
			DestPortName:   gaia1Port,
			Order:          ibc.Unordered,
			Version:        "ics20-1",
		}))

		// Wait for another block before retrieving the channels and querying for them.
		require.NoError(t, testutil.WaitForBlocks(ctx, 1, gaia0, gaia1))

		channels, err := r.GetChannels(ctx, eRep, gaia0ChainID)
		require.NoError(t, err)
		require.Len(t, channels, 1)

		gaia0ChannelID = channels[0].ChannelID
		gaia1ChannelID = channels[0].Counterparty.ChannelID

		// OpenInit happens on first chain.
		const qChannelOpenInit = `SELECT
port_id, counterparty_port_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenInit" AND chain_id = ?
`
		var portID, counterpartyPortID string
		require.NoError(t, db.QueryRow(qChannelOpenInit, gaia0ChainID).Scan(&portID, &counterpartyPortID))
		require.Equal(t, portID, gaia0Port)
		require.Equal(t, counterpartyPortID, gaia1Port)

		// OpenTry happens on second chain.
		const qChannelOpenTry = `SELECT
port_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenTry" AND chain_id = ?
`
		var counterpartyChannelID string
		require.NoError(t, db.QueryRow(qChannelOpenTry, gaia1ChainID).Scan(&portID, &counterpartyPortID, &counterpartyChannelID))
		require.Equal(t, portID, gaia1Port)
		require.Equal(t, counterpartyPortID, gaia0Port)
		require.Equal(t, counterpartyChannelID, gaia0ChannelID)

		// OpenAck happens on first chain again.
		const qChannelOpenAck = `SELECT
port_id, channel_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenAck" AND chain_id = ?
`
		var channelID string
		require.NoError(t, db.QueryRow(qChannelOpenAck, gaia0ChainID).Scan(&portID, &channelID, &counterpartyChannelID))
		require.Equal(t, portID, gaia0Port)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)

		// OpenConfirm happens on second chain again.
		const qChannelOpenConfirm = `SELECT
port_id, channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenConfirm" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qChannelOpenConfirm, gaia1ChainID).Scan(&portID, &channelID))
		require.Equal(t, portID, gaia1Port)
		require.Equal(t, channelID, gaia1ChannelID)
	})
	if t.Failed() {
		return
	}

	t.Run("initiate transfer", func(t *testing.T) {
		// Build the faucet address for gaia1, so that gaia0 can send it a transfer.
		g1FaucetAddrBytes, err := gaia1.GetAddress(ctx, interchaintest.FaucetAccountKeyName)
		require.NoError(t, err)
		gaia1FaucetAddr, err := types.Bech32ifyAddressBytes(gaia1.Config().Bech32Prefix, g1FaucetAddrBytes)
		require.NoError(t, err)

		// Send the IBC transfer. Relayer isn't running, so this will just create a MsgTransfer.
		const txAmount = 13579 // Arbitrary amount that is easy to find in logs.
		transfer := ibc.WalletAmount{
			Address: gaia1FaucetAddr,
			Denom:   gaia0.Config().Denom,
			Amount:  math.NewInt(txAmount),
		}
		tx, err := gaia0.SendIBCTransfer(ctx, gaia0ChannelID, interchaintest.FaucetAccountKeyName, transfer, ibc.TransferOptions{})
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		const qMsgTransfer = `SELECT
port_id, channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.applications.transfer.v1.MsgTransfer" AND chain_id = ?
`
		var portID, channelID string
		require.NoError(t, db.QueryRow(qMsgTransfer, gaia0ChainID).Scan(&portID, &channelID))
		require.Equal(t, portID, gaia0Port)
		require.Equal(t, channelID, gaia0ChannelID)
	})
	if t.Failed() {
		return
	}

	if !rf.Capabilities()[relayer.Flush] {
		t.Skip("cannot continue due to missing capability Flush")
	}

	t.Run("relay", func(t *testing.T) {
		require.NoError(t, r.Flush(ctx, eRep, pathName, gaia0ChannelID))
		require.NoError(t, testutil.WaitForBlocks(ctx, 5, gaia0))

		const qMsgRecvPacket = `SELECT
port_id, channel_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgRecvPacket" AND chain_id = ?
`

		var portID, channelID, counterpartyPortID, counterpartyChannelID string

		require.NoError(t, db.QueryRow(qMsgRecvPacket, gaia1ChainID).Scan(&portID, &channelID, &counterpartyPortID, &counterpartyChannelID))

		require.Equal(t, portID, gaia0Port)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, counterpartyPortID, gaia1Port)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)

		const qMsgAck = `SELECT
port_id, channel_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgAcknowledgement" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qMsgAck, gaia0ChainID).Scan(&portID, &channelID, &counterpartyPortID, &counterpartyChannelID))

		require.Equal(t, portID, gaia0Port)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, counterpartyPortID, gaia1Port)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)
	})
}
