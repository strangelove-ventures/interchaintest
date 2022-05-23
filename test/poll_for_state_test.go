package test

import (
	"context"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type mockAcker struct {
	HeightErr       error
	HeightCallCount int
	CurrentHeight   int

	GotAckHeight uint64
	Packet       ibc.PacketAcknowledgement
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

func (m *mockAcker) Acknowledgement(ctx context.Context, height uint64) (ibc.PacketAcknowledgement, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotAckHeight = height
	return m.Packet, m.AckErr
}

//func TestPollForAck(t *testing.T) {
//	ctx := context.Background()
//
//	t.Run("happy path", func(t *testing.T) {
//		acker := mockAcker{Packet: ibc.PacketAcknowledgement{Acknowledgement: []byte(`test`)}}
//		var cbCalled bool
//		err := PollForAck(ctx, 10, &acker, func(ack ibc.PacketAcknowledgement) bool {
//			require.Equal(t, acker.Packet, ack)
//			cbCalled = true
//			return true
//		})
//
//		require.NoError(t, err)
//		require.True(t, cbCalled)
//		require.EqualValues(t, 1, acker.GotAckHeight)
//	})
//
//	t.Run("height error", func(t *testing.T) {
//		acker := mockAcker{HeightErr: errors.New("height go boom")}
//		err := PollForAck(ctx, 10, &acker, func(ibc.PacketAcknowledgement) bool {
//			panic("should not be called")
//		})
//
//		require.Error(t, err)
//		require.EqualError(t, err, "height go boom")
//		require.Equal(t, 1, acker.HeightCallCount)
//	})
//
//	t.Run("height timeout", func(t *testing.T) {
//		acker := mockAcker{
//			CurrentHeight: 10,
//		}
//		err := PollForAck(ctx, 4, &acker, func(ibc.PacketAcknowledgement) bool {
//			return false
//		})
//
//		require.Error(t, err)
//		require.EqualError(t, err, "height timeout 4 reached")
//		require.Equal(t, 5, acker.HeightCallCount)
//	})
//
//	t.Run("height timeout with error", func(t *testing.T) {
//		acker := mockAcker{
//			CurrentHeight: 10,
//			AckErr:        errors.New("ack go boom"),
//		}
//		err := PollForAck(ctx, 4, &acker, func(ibc.PacketAcknowledgement) bool {
//			panic("should not be called")
//		})
//
//		require.Error(t, err)
//		require.EqualError(t, err, "height timeout 4 reached: ack go boom")
//		require.Greater(t, acker.CurrentHeight, 10)
//		require.Equal(t, 5, acker.HeightCallCount)
//	})
//}
