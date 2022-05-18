package ibc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPacket_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		packet := &Packet{
			Sequence:         1,
			TimeoutHeight:    "100",
			TimeoutTimestamp: 0,
			SourcePort:       "transfer",
			SourceChannel:    "channel-0",
			DestPort:         "transfer",
			DestChannel:      "channel-1",
			Data:             []byte(`fake data`),
		}

		require.NoError(t, packet.Validate())

		packet.TimeoutHeight = ""
		packet.TimeoutTimestamp = 1

		require.NoError(t, packet.Validate())
	})

	t.Run("invalid", func(t *testing.T) {
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
				"invalid source port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "@"},
				"invalid source port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer"},
				"invalid source channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "@"},
				"invalid source channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0"},
				"invalid destination port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "@"},
				"invalid destination port:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer"},
				"invalid destination channel:",
			},
			{
				Packet{Sequence: 1, SourcePort: "transfer", SourceChannel: "channel-0", DestPort: "transfer", DestChannel: "@"},
				"invalid destination channel:",
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
