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

	GotAckHeights []uint64
	Acks          []ibc.PacketAcknowledgement
	AckErr        error
}

func (m *mockAcker) Height(ctx context.Context) (uint64, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.HeightCallCount++
	defer func() { m.CurrentHeight++ }()
	return uint64(m.CurrentHeight), m.HeightErr
}

func (m *mockAcker) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotAckHeights = append(m.GotAckHeights, height)
	return m.Acks, m.AckErr
}

func TestPollForAck(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		chain := mockAcker{CurrentHeight: 1, Acks: []ibc.PacketAcknowledgement{
			{Packet: ibc.Packet{Sequence: 44, SourceChannel: "other"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "found"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "ignore"}},
		}}
		got, err := PollForAck(ctx, &chain, 3, 5, ibc.Packet{Sequence: 33, SourceChannel: "found"})

		require.NoError(t, err)
		require.Equal(t, "found", got.Packet.SourceChannel)
		require.EqualValues(t, 33, got.Packet.Sequence)

		require.Equal(t, []uint64{3}, chain.GotAckHeights)
		require.Equal(t, 3, chain.HeightCallCount)
	})

	t.Run("height error", func(t *testing.T) {
		chain := mockAcker{HeightErr: errors.New("height go boom")}
		_, err := PollForAck(ctx, &chain, 3, 5, ibc.Packet{})

		require.Error(t, err)
		require.EqualError(t, err, "height go boom")
	})

	t.Run("find acks error", func(t *testing.T) {
		chain := mockAcker{CurrentHeight: 1, AckErr: errors.New("ack go boom")}
		_, err := PollForAck(ctx, &chain, 1, 10, ibc.Packet{})

		require.Error(t, err)
		require.EqualError(t, err, "ack go boom")
		require.Len(t, chain.GotAckHeights, 10)
	})

	t.Run("not found", func(t *testing.T) {
		chain := mockAcker{CurrentHeight: 1}
		_, err := PollForAck(ctx, &chain, 1, 3, ibc.Packet{})

		require.Error(t, err)
		require.EqualError(t, err, "acknowledgement not found")
		require.Equal(t, []uint64{1, 2, 3}, chain.GotAckHeights)
	})

	t.Run("invalid args", func(t *testing.T) {
		require.Panics(t, func() {
			_, _ = PollForAck(ctx, &mockAcker{}, 10, 1, ibc.Packet{})
		})
	})
}
