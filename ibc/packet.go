package ibc

import (
	"errors"
	"fmt"

	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"go.uber.org/multierr"
)

type Nanoseconds uint64

// Packet is a packet sent over an IBC channel as defined in ICS-4.
// See: https://github.com/cosmos/ibc/blob/52a9094a5bc8c5275e25c19d0b2d9e6fd80ba31c/spec/core/ics-004-channel-and-packet-semantics/README.md
// Proto defined at: github.com/cosmos/ibc-go/v3@v3.0.0/proto/ibc/core/channel/v1/tx.proto
type Packet struct {
	Sequence      uint64 // the order of sends and receives, where a packet with an earlier sequence number must be sent and received before a packet with a later sequence number
	SourcePort    string // the port on the sending chain
	SourceChannel string // the channel end on the sending chain
	DestPort      string // the port on the receiving chain
	DestChannel   string // the channel end on the receiving chain
	Data          []byte // an opaque value which can be defined by the application logic of the associated modules
	TimeoutHeight string // a consensus height on the destination chain after which the packet will no longer be processed, and will instead count as having timed-out

	// Indicates a timestamp (in nanoseconds) on the destination chain after which the packet will no longer be processed, and will instead count as having timed-out.
	// The IBC spec does not indicate the unit of time. However, ibc-go's protos define it as nanoseconds.
	TimeoutTimestamp Nanoseconds
}

// Validate returns an error if the packet is not well-formed.
func (packet Packet) Validate() error {
	var merr error
	addErr := func(err error) {
		merr = multierr.Append(merr, err)
	}
	if packet.Sequence == 0 {
		addErr(errors.New("packet sequence cannot be 0"))
	}
	if err := host.PortIdentifierValidator(packet.SourcePort); err != nil {
		addErr(fmt.Errorf("invalid packet source port: %w", err))
	}
	if err := host.ChannelIdentifierValidator(packet.SourceChannel); err != nil {
		addErr(fmt.Errorf("invalid packet source channel: %w", err))
	}
	if err := host.PortIdentifierValidator(packet.DestPort); err != nil {
		addErr(fmt.Errorf("invalid packet destination port: %w", err))
	}
	if err := host.ChannelIdentifierValidator(packet.DestChannel); err != nil {
		addErr(fmt.Errorf("invalid packet destination channel: %w", err))
	}
	if len(packet.TimeoutHeight) == 0 && packet.TimeoutTimestamp <= 0 {
		addErr(errors.New("packet timeout height and packet timeout timestamp cannot both be 0"))
	}
	if len(packet.Data) == 0 {
		addErr(errors.New("packet data bytes cannot be empty"))
	}
	return merr
}
