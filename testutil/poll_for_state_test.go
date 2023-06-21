package testutil

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v5/ibc"
	"github.com/stretchr/testify/require"
)

type mockChain struct {
	HeightErr       error
	HeightCallCount int
	CurrentHeight   int

	GotHeights []uint64

	FoundAcks []ibc.PacketAcknowledgement
	AckErr    error

	FoundTimeouts []ibc.PacketTimeout
	TimeoutErr    error
}

func (m *mockChain) Height(ctx context.Context) (uint64, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.HeightCallCount++
	defer func() { m.CurrentHeight++ }()
	return uint64(m.CurrentHeight), m.HeightErr
}

func (m *mockChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotHeights = append(m.GotHeights, height)
	return m.FoundAcks, m.AckErr
}

func (m *mockChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotHeights = append(m.GotHeights, height)
	return m.FoundTimeouts, m.TimeoutErr
}

func TestPollForAck(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1, FoundAcks: []ibc.PacketAcknowledgement{
			{Packet: ibc.Packet{Sequence: 44, SourceChannel: "other"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "found"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "ignore"}},
		}}
		got, err := PollForAck(ctx, &chain, 3, 5, ibc.Packet{Sequence: 33, SourceChannel: "found"})

		require.NoError(t, err)
		require.Equal(t, "found", got.Packet.SourceChannel)
		require.EqualValues(t, 33, got.Packet.Sequence)

		require.Equal(t, []uint64{3}, chain.GotHeights)
		require.Equal(t, 3, chain.HeightCallCount)
	})

	t.Run("height error", func(t *testing.T) {
		chain := mockChain{HeightErr: errors.New("height go boom")}
		_, err := PollForAck(ctx, &chain, 3, 5, ibc.Packet{})

		require.Error(t, err)
		require.EqualError(t, err, "height go boom")
	})

	t.Run("find acks error", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1, AckErr: errors.New("ack go boom")}
		_, err := PollForAck(ctx, &chain, 1, 10, ibc.Packet{})

		require.Error(t, err)
		require.Contains(t, err.Error(), "ack go boom")
		require.Len(t, chain.GotHeights, 10)
	})

	t.Run("not found", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1, FoundAcks: []ibc.PacketAcknowledgement{
			{Packet: ibc.Packet{Sequence: 10}},
		}}
		_, err := PollForAck(ctx, &chain, 1, 3, ibc.Packet{Sequence: 5})

		require.Error(t, err)
		require.EqualError(t, err, "not found")
		require.ErrorIs(t, err, ErrNotFound)
		require.Equal(t, []uint64{1, 2, 3}, chain.GotHeights)

		longErr := fmt.Sprintf("%+v", err)
		require.Contains(t, longErr, "not found")
		require.Regexp(t, `(?s)target packet:.*Sequence.*5`, longErr)
		require.Contains(t, longErr, "searched:")
	})

	t.Run("invalid args", func(t *testing.T) {
		require.Panics(t, func() {
			_, _ = PollForAck(ctx, &mockChain{}, 10, 1, ibc.Packet{})
		})
	})
}

func TestPollForTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1, FoundTimeouts: []ibc.PacketTimeout{
			{Packet: ibc.Packet{Sequence: 44, SourceChannel: "other"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "found"}},
			{Packet: ibc.Packet{Sequence: 33, SourceChannel: "ignore"}},
		}}
		got, err := PollForTimeout(ctx, &chain, 3, 5, ibc.Packet{Sequence: 33, SourceChannel: "found"})

		require.NoError(t, err)
		require.Equal(t, "found", got.Packet.SourceChannel)
		require.EqualValues(t, 33, got.Packet.Sequence)

		require.Equal(t, []uint64{3}, chain.GotHeights)
		require.Equal(t, 3, chain.HeightCallCount)
	})

	t.Run("height error", func(t *testing.T) {
		chain := mockChain{HeightErr: errors.New("height go boom")}
		_, err := PollForTimeout(ctx, &chain, 3, 5, ibc.Packet{})

		require.Error(t, err)
		require.EqualError(t, err, "height go boom")
	})

	t.Run("find timeouts error", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1, TimeoutErr: errors.New("timeout go boom")}
		_, err := PollForTimeout(ctx, &chain, 1, 10, ibc.Packet{})

		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout go boom")
		require.Len(t, chain.GotHeights, 10)
	})

	t.Run("not found", func(t *testing.T) {
		chain := mockChain{CurrentHeight: 1}
		_, err := PollForTimeout(ctx, &chain, 1, 3, ibc.Packet{})

		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
		require.Equal(t, []uint64{1, 2, 3}, chain.GotHeights)
	})

	t.Run("invalid args", func(t *testing.T) {
		require.Panics(t, func() {
			_, _ = PollForTimeout(ctx, &mockChain{}, 10, 1, ibc.Packet{})
		})
	})
}
