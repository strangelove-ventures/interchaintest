package presenter

import (
	"strconv"
	"strings"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// CosmosMessage presents a blockdb.CosmosMessageResult.
type CosmosMessage struct {
	Result blockdb.CosmosMessageResult
}

func (msg CosmosMessage) Height() string { return strconv.FormatInt(msg.Result.Height, 10) }

// Index is the message's ordered position within the tx.
func (msg CosmosMessage) Index() string { return strconv.Itoa(msg.Result.Index) }

// Type is a URI for the proto definition, e.g. /ibc.core.client.v1.MsgCreateClient
func (msg CosmosMessage) Type() string { return msg.Result.Type }

func (msg CosmosMessage) ClientChain() string { return msg.Result.ClientChainID.String }

func (msg CosmosMessage) Clients() string {
	return msg.srcCounterpartyPair(msg.Result.ClientID.String, msg.Result.CounterpartyClientID.String)
}

func (msg CosmosMessage) Connections() string {
	return msg.srcCounterpartyPair(msg.Result.ConnID.String, msg.Result.CounterpartyConnID.String)
}

func (msg CosmosMessage) Channels() string {
	join := func(channel, port string) string {
		if channel+port == "" {
			return ""
		}
		return channel + ":" + port
	}
	return msg.srcCounterpartyPair(
		join(msg.Result.ChannelID.String, msg.Result.PortID.String),
		join(msg.Result.CounterpartyChannelID.String, msg.Result.CounterpartyPortID.String),
	)
}

func (msg CosmosMessage) srcCounterpartyPair(source, counterparty string) string {
	if source != "" {
		source += " (source)"
	}
	if counterparty != "" {
		counterparty += " (counterparty)"
	}
	return strings.TrimSpace(source + " " + counterparty)
}
