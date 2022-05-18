package test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockChainHeighter struct {
	CurHeight uint64
	Err       error
}

func (m *mockChainHeighter) Height(ctx context.Context) (uint64, error) {
	if ctx == nil {
		panic("nil context")
	}
	atomic.AddUint64(&m.CurHeight, 1)
	return m.CurHeight, m.Err
}

func TestWaitForBlocks(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var (
			startHeight1 uint64 = 10
			chain1              = mockChainHeighter{CurHeight: startHeight1}
			startHeight2 uint64 = 5
			chain2              = mockChainHeighter{CurHeight: startHeight2}
		)

		const delta = 5
		err := WaitForBlocks(context.Background(), delta, &chain1, &chain2)

		require.NoError(t, err)
		// +1 accounts for initial increment
		require.InDelta(t, startHeight1+1, chain1.CurHeight, delta)
		require.InDelta(t, startHeight2+1, chain2.CurHeight, delta)
	})

	t.Run("zero state", func(t *testing.T) {
		require.Panics(t, func() {
			_ = WaitForBlocks(context.Background(), 100)
		})
	})

	t.Run("error", func(t *testing.T) {
		errMock := mockChainHeighter{Err: errors.New("boom")}
		const delta = 1
		err := WaitForBlocks(context.Background(), delta, &mockChainHeighter{}, &errMock)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}
