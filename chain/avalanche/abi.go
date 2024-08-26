package avalanche

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/coreth/precompile/contract"
)

var (
	//go:embed abi.json
	rawABI string

	abi = contract.ParseABI(rawABI)
)

type Height struct {
	RevisionNumber *big.Int
	RevisionHeight *big.Int
}

type MsgSendPacket struct {
	ChannelCapability *big.Int
	SourcePort        string
	SourceChannel     string
	TimeoutHeight     Height
	TimeoutTimestamp  *big.Int
	Data              []byte
}

type FungibleTokenPacketData struct {
	// the token denomination to be transferred
	Denom string `json:"denom"`
	// the token amount to be transferred
	Amount string `json:"amount"`
	// the sender address
	Sender string `json:"sender"`
	// the recipient address on the destination chain
	Receiver string `json:"receiver"`
	// optional memo
	Memo string `json:"memo,omitempty"`
}

func packSendPacket(msg MsgSendPacket) ([]byte, error) {
	return abi.Pack("sendPacket", msg.ChannelCapability, msg.SourcePort, msg.SourceChannel, msg.TimeoutHeight, msg.TimeoutTimestamp, msg.Data)
}
