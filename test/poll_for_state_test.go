package test

import (
	"context"
	"errors"
	"testing"

	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/stretchr/testify/require"
)

type mockAcker struct {
	HeightErr       error
	HeightCallCount int
	CurrentHeight   int

	GotAckHeight uint64
	Packet       ibc.PacketAcknowledgment
	AckErr       error
}

func (m *mockAcker) Height(ctx context.Context) (uint64, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.HeightCallCount++
	m.CurrentHeight++
	return uint64(m.CurrentHeight), m.HeightErr
}

func (m *mockAcker) AcknowledgementPacket(ctx context.Context, height uint64) (ibc.PacketAcknowledgment, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotAckHeight = height
	return m.Packet, m.AckErr
}

func TestPollForAcks(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		acker := mockAcker{Packet: ibc.PacketAcknowledgment{Acknowledgement: []byte(`test`)}}
		got, err := PollForAcks(ctx, 10, &acker)

		require.NoError(t, err)
		require.Equal(t, acker.Packet, got)
		require.EqualValues(t, 1, acker.GotAckHeight)
	})

	t.Run("height error", func(t *testing.T) {
		acker := mockAcker{HeightErr: errors.New("height go boom")}
		_, err := PollForAcks(ctx, 10, &acker)

		require.Error(t, err)
		require.EqualError(t, err, "height go boom")
		require.Equal(t, 1, acker.HeightCallCount)
	})

	t.Run("height timeout", func(t *testing.T) {
		acker := mockAcker{
			CurrentHeight: 10,
			AckErr:        errors.New("ack go boom"),
		}
		_, err := PollForAcks(ctx, 4, &acker)

		require.Error(t, err)
		require.EqualError(t, err, "height timeout 4 reached: ack go boom")
		require.Greater(t, acker.CurrentHeight, 10)
		require.Equal(t, 5, acker.HeightCallCount)
	})
}
