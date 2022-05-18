package ibc

import (
	"errors"
	"fmt"
	"time"

	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
)

// Packet is a packet sent over an IBC channel as defined in ICS-4.
// See: https://github.com/cosmos/ibc/blob/master/spec/core/ics-004-channel-and-packet-semantics/README.md
// Proto defined at: github.com/cosmos/ibc-go/v3@v3.0.0/proto/ibc/core/channel/v1/tx.proto
type Packet struct {
	Sequence         uint64        // the order of sends and receives, where a packet with an earlier sequence number must be sent and received before a packet with a later sequence number
	SourcePort       string        // the port on the sending chain
	SourceChannel    string        // the channel end on the sending chain
	DestPort         string        // the port on the receiving chain
	DestChannel      string        // the channel end on the receiving chain
	Data             []byte        // an opaque value which can be defined by the application logic of the associated modules
	TimeoutHeight    string        // a consensus height on the destination chain after which the packet will no longer be processed, and will instead count as having timed-out
	TimeoutTimestamp time.Duration // indicates a timestamp on the destination chain after which the packet will no longer be processed, and will instead count as having timed-out
}

// Validate returns an error if the packet is not well-formed.
func (packet Packet) Validate() error {
	if packet.Sequence == 0 {
		return errors.New("packet sequence cannot be 0")
	}
	if err := host.PortIdentifierValidator(packet.SourcePort); err != nil {
		return fmt.Errorf("invalid source port: %w", err)
	}
	if err := host.ChannelIdentifierValidator(packet.SourceChannel); err != nil {
		return fmt.Errorf("invalid source channel: %w", err)
	}
	if err := host.PortIdentifierValidator(packet.DestPort); err != nil {
		return fmt.Errorf("invalid destination port: %w", err)
	}
	if err := host.ChannelIdentifierValidator(packet.DestChannel); err != nil {
		return fmt.Errorf("invalid destination channel: %w", err)
	}
	if len(packet.TimeoutHeight) == 0 && packet.TimeoutTimestamp <= 0 {
		return errors.New("packet timeout height and packet timeout timestamp cannot both be 0")
	}
	if len(packet.Data) == 0 {
		return errors.New("packet data bytes cannot be empty")
	}
	return nil
}

type PacketAckType int

const (
	UnknownPacketAckType PacketAckType = iota
	AcknowledgedPacketAckType
	TimeoutPacketAckType
)

// PacketAcknowledgement is received by the sending chain from the counterparty.
// SEE: https://github.com/cosmos/ibc/blob/master/spec/core/ics-004-channel-and-packet-semantics/README.md#writing-acknowledgements
type PacketAcknowledgement struct {
	Packet          Packet
	Type            PacketAckType
	Acknowledgement []byte // arbitrary data defined by and for use by the application
}
