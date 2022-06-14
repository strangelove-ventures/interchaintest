package presenter

import (
	"testing"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
	"github.com/stretchr/testify/require"
)

func TestCosmosMessage(t *testing.T) {
	t.Parallel()

	t.Run("non-variable fields", func(t *testing.T) {
		res := blockdb.CosmosMessageResult{
			Height: 55,
			Index:  13,
			Type:   "/ibc.MsgFoo",
		}
		pres := CosmosMessage{res}

		require.Equal(t, "55", pres.Height())
		require.Equal(t, "13", pres.Index())
		require.Equal(t, "/ibc.MsgFoo", pres.Type())
	})

	//t.Run("ibc details", func(t *testing.T) {
	//	for _, tt := range []struct {
	//		Result blockdb.CosmosMessageResult
	//		Want   string
	//	}{
	//		{
	//			// zero state
	//			blockdb.CosmosMessageResult{},
	//			"",
	//		},
	//		{
	//			blockdb.CosmosMessageResult{ClientChainID: sql.NullString{String: "other-chain", Valid: true}},
	//			"ClientChain: other-chain",
	//		},
	//		{
	//			blockdb.CosmosMessageResult{
	//				ClientID:             sql.NullString{String: "tendermint-1", Valid: true},
	//				CounterpartyClientID: sql.NullString{String: "tendermint-2", Valid: true},
	//			},
	//			"Client: tendermint-1 · Counterparty Client: tendermint-2",
	//		},
	//		{
	//			blockdb.CosmosMessageResult{
	//				ConnID:             sql.NullString{String: "conn-1", Valid: true},
	//				CounterpartyConnID: sql.NullString{String: "conn-2", Valid: true},
	//			},
	//			"Connection: conn-1 · Counterparty Connection: conn-2",
	//		},
	//		{
	//			blockdb.CosmosMessageResult{
	//				PortID:                sql.NullString{String: "port-1", Valid: true},
	//				ChannelID:             sql.NullString{String: "chan-1", Valid: true},
	//				CounterpartyPortID:    sql.NullString{String: "port-2", Valid: true},
	//				CounterpartyChannelID: sql.NullString{String: "chan-2", Valid: true},
	//			},
	//			"Channel: chan-1 · Port: port-1 · Counterparty Channel: chan-2 · Counterparty Port: port-2",
	//		},
	//	} {
	//		pres := CosmosMessage{tt.Result}
	//		require.Equal(t, tt.Want, pres.IBCDetails(), tt)
	//	}
	//})
}
