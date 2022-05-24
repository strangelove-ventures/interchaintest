package ibc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func validPacket() Packet {
	return Packet{
		Sequence:         1,
		TimeoutHeight:    "100",
		TimeoutTimestamp: 0,
		SourcePort:       "transfer",
		SourceChannel:    "channel-0",
		DestPort:         "transfer",
		DestChannel:      "channel-1",
		Data:             []byte(`fake data`),
	}
}

func TestPacket_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		packet := validPacket()

		require.NoError(t, packet.Validate())

		packet.TimeoutHeight = ""
		packet.TimeoutTimestamp = 1

		require.NoError(t, packet.Validate())
	})

	t.Run("invalid", func(t *testing.T) {
		var empty Packet
		merr := empty.Validate()

		require.Error(t, merr)
		require.Greater(t, len(multierr.Errors(merr)), 1)

		for _, tt := range []struct {
			Packet  Packet
			WantErr string
		}{
			{
				Packet{Sequence: 0},
				"packet sequence cannot be 0",
			},
			{
				Packet{Sequence: 1},
				"invalid packet source port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "@"},
				"invalid packet source port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer"},
				"invalid packet source channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "@"},
				"invalid packet source channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0"},
				"invalid packet destination port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "@"},
				"invalid packet destination port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer"},
				"invalid packet destination channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer", DestChannel: "@"},
				"invalid packet destination channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer", DestChannel: "channel-0"},
				"packet timeout height and packet timeout timestamp cannot both be 0",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer", DestChannel: "channel-0", TimeoutHeight: "100"},
				"packet data bytes cannot be empty",
			},
		} {
			err := tt.Packet.Validate()
			require.Error(t, err, tt)
			require.Contains(t, err.Error(), tt.WantErr, tt)
		}
	})
}

func TestPacket_Equal(t *testing.T) {
	for _, tt := range []struct {
		Left, Right Packet
		WantEqual   bool
	}{
		{validPacket(), validPacket(), true},
		{Packet{}, Packet{}, true},

		{validPacket(), Packet{}, false},
		{Packet{Data: []byte(`left`)}, Packet{Data: []byte(`two`)}, false},
		{Packet{Sequence: 1}, Packet{Sequence: 2}, false},
	} {
		require.Equal(t, tt.WantEqual, tt.Left.Equal(tt.Right), tt)
		require.Equal(t, tt.WantEqual, tt.Right.Equal(tt.Left), tt)

		require.True(t, tt.Left.Equal(tt.Left))
		require.True(t, tt.Right.Equal(tt.Right))
	}
}

func TestPacketAcknowledgment_Validate(t *testing.T) {
	var ack PacketAcknowledgement
	err := ack.Validate()
	require.Error(t, err)

	ack.Packet = validPacket()
	err = ack.Validate()
	require.Error(t, err)
	require.EqualError(t, err, "packet acknowledgement cannot be empty")

	ack.Acknowledgement = []byte(`ack`)
	err = ack.Validate()
	require.NoError(t, err)
}

func TestPacketTimeout_Validate(t *testing.T) {
	var timeout PacketTimeout
	require.Error(t, timeout.Validate())

	timeout.Packet = validPacket()
	require.NoError(t, timeout.Validate())
}
