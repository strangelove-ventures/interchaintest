package test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockChainHeighter struct {
	CurHeight uint64
	Err       error
	Sleep     time.Duration
}

func (m *mockChainHeighter) Height(ctx context.Context) (uint64, error) {
	if ctx == nil {
		panic("nil context")
	}
	if m.Sleep > 0 {
		time.Sleep(m.Sleep)
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
		require.EqualValues(t, startHeight1+delta+1, chain1.CurHeight) // +1 accounts for initial fetch of the height
		require.EqualValues(t, startHeight2+delta+1, chain2.CurHeight)
	})

	t.Run("zero state", func(t *testing.T) {
		err := WaitForBlocks(context.Background(), 100)

		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		errMock := mockChainHeighter{Err: errors.New("boom")}
		const delta = 1
		err := WaitForBlocks(context.Background(), delta, &mockChainHeighter{}, &errMock)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})

	t.Run("context done", func(t *testing.T) {
		const delta = 100
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := WaitForBlocks(ctx, delta, &mockChainHeighter{Sleep: time.Minute}, &mockChainHeighter{})

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}
