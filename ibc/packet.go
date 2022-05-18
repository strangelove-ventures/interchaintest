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
		return fmt.Errorf("invalid packet source port: %w", err)
	}
	if err := host.ChannelIdentifierValidator(packet.SourceChannel); err != nil {
		return fmt.Errorf("invalid packet source channel: %w", err)
	}
	if err := host.PortIdentifierValidator(packet.DestPort); err != nil {
		return fmt.Errorf("invalid packet destination port: %w", err)
	}
	if err := host.ChannelIdentifierValidator(packet.DestChannel); err != nil {
		return fmt.Errorf("invalid packet destination channel: %w", err)
	}
	if len(packet.TimeoutHeight) == 0 && packet.TimeoutTimestamp <= 0 {
		return errors.New("packet timeout height and packet timeout timestamp cannot both be 0")
	}
	if len(packet.Data) == 0 {
		return errors.New("packet data bytes cannot be empty")
	}
	return nil
}

// PacketAcknowledgement is received by the sending chain from the counterparty acknowledging the packet
// was successfully processed.
// See: https://github.com/cosmos/ibc/blob/master/spec/core/ics-004-channel-and-packet-semantics/README.md#writing-acknowledgements
// Proto defined at: github.com/cosmos/ibc-go/v3@v3.0.0/proto/ibc/core/channel/v1/tx.proto
// TODO(nix 05-18-2022): Add missing fields such as proof and height
type PacketAcknowledgement struct {
	Packet          Packet
	Acknowledgement []byte // arbitrary data defined by and for use by the application
}

// Validate returns an error if the acknowledgement is not well-formed.
func (ack PacketAcknowledgement) Validate() error {
	if err := ack.Packet.Validate(); err != nil {
		return err
	}
	if len(ack.Acknowledgement) == 0 {
		return errors.New("packet acknowledgement bytes cannot be empty")
	}
	return nil
}

// PacketTimeout is received by the sending chain from the counterparty if the packet met a height or timestamp
// timeout threshold.
// TODO(nix 05-18-2022): Add missing fields such as proof and height
type PacketTimeout struct {
	Packet Packet
}

// Validate returns an error if the timeout is not well-formed.
func (timeout PacketTimeout) Validate() error {
	return timeout.Packet.Validate()
}
