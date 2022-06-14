package presenter

import (
	"database/sql"
	"testing"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
	"github.com/stretchr/testify/require"
)

func TestCosmosMessage(t *testing.T) {
	t.Parallel()

	t.Run("non-variable fields", func(t *testing.T) {
		res := blockdb.CosmosMessageResult{
			Height:        55,
			Index:         13,
			Type:          "/ibc.MsgFoo",
			ClientChainID: sql.NullString{String: "chain1"},
		}
		pres := CosmosMessage{res}

		require.Equal(t, "55", pres.Height())
		require.Equal(t, "13", pres.Index())
		require.Equal(t, "/ibc.MsgFoo", pres.Type())
		require.Equal(t, "chain1", pres.ClientChain())
		require.Empty(t, pres.Clients())
		require.Empty(t, pres.Connections())
		require.Empty(t, pres.Channels())
	})

	t.Run("ibc details", func(t *testing.T) {
		for _, tt := range []struct {
			Result          blockdb.CosmosMessageResult
			WantClients     string
			WantConnections string
			WantChannels    string
		}{
			{
				blockdb.CosmosMessageResult{
					ClientID:              sql.NullString{String: "tendermint-1", Valid: true},
					CounterpartyClientID:  sql.NullString{String: "tendermint-2", Valid: true},
					ConnID:                sql.NullString{String: "conn-1", Valid: true},
					CounterpartyConnID:    sql.NullString{String: "conn-2", Valid: true},
					PortID:                sql.NullString{String: "port-1", Valid: true},
					ChannelID:             sql.NullString{String: "chan-1", Valid: true},
					CounterpartyPortID:    sql.NullString{String: "port-2", Valid: true},
					CounterpartyChannelID: sql.NullString{String: "chan-2", Valid: true},
				},
				"tendermint-1 (source) tendermint-2 (counterparty)",
				"conn-1 (source) conn-2 (counterparty)",
				"chan-1:port-1 (source) chan-2:port-2 (counterparty)",
			},
			{
				blockdb.CosmosMessageResult{
					ClientID:  sql.NullString{String: "tendermint-1", Valid: true},
					ConnID:    sql.NullString{String: "conn-1", Valid: true},
					PortID:    sql.NullString{String: "port-1", Valid: true},
					ChannelID: sql.NullString{String: "chan-1", Valid: true},
				},
				"tendermint-1 (source)",
				"conn-1 (source)",
				"chan-1:port-1 (source)",
			},
			{
				blockdb.CosmosMessageResult{
					CounterpartyClientID:  sql.NullString{String: "tendermint-2", Valid: true},
					CounterpartyConnID:    sql.NullString{String: "conn-2", Valid: true},
					CounterpartyPortID:    sql.NullString{String: "port-2", Valid: true},
					CounterpartyChannelID: sql.NullString{String: "chan-2", Valid: true},
				},
				"tendermint-2 (counterparty)",
				"conn-2 (counterparty)",
				"chan-2:port-2 (counterparty)",
			},
		} {
			pres := CosmosMessage{tt.Result}
			require.Equal(t, tt.WantClients, pres.Clients(), tt)
			require.Equal(t, tt.WantConnections, pres.Connections(), tt)
			require.Equal(t, tt.WantChannels, pres.Channels(), tt)
		}
	})
}
