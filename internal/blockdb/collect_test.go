package blockdb

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type mockTxFinder func(ctx context.Context, height uint64) ([]Tx, error)

func (f mockTxFinder) FindTxs(ctx context.Context, height uint64) ([]Tx, error) {
	return f(ctx, height)
}

type mockBlockSaver func(ctx context.Context, height uint64, txs []Tx) error

func (f mockBlockSaver) SaveBlock(ctx context.Context, height uint64, txs []Tx) error {
	return f(ctx, height, txs)
}

func TestCollector_Collect(t *testing.T) {
	nopLog := zap.NewNop()

	t.Run("happy path", func(t *testing.T) {
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([]Tx, error) {
			if height == 0 {
				panic("zero height")
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				if height > 3 {
					return nil, nil
				}
				return []Tx{{Data: []byte(strconv.FormatUint(height, 10))}}, nil
			}
		})

		var (
			currentHeight int64
			savedHeights  []int
			savedTxs      [][]Tx
		)
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs []Tx) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				savedHeights = append(savedHeights, int(height))
				savedTxs = append(savedTxs, txs)
			}
			atomic.SwapInt64(&currentHeight, int64(height))
			return nil
		})

		collector := NewCollector(nopLog, finder, saver, time.Nanosecond)
		defer collector.Stop()

		ctx, cancel := context.WithCancel(context.Background())
		var eg errgroup.Group

		eg.Go(func() error {
			collector.Collect(ctx)
			return nil
		})
		eg.Go(func() error {
			for atomic.LoadInt64(&currentHeight) <= 3 {
			}
			cancel()
			return nil
		})

		require.NoError(t, eg.Wait())
		require.Equal(t, []int{1, 2, 3}, savedHeights[:3])
		require.Equal(t, "1", string(savedTxs[0][0].Data))
		require.Equal(t, "2", string(savedTxs[1][0].Data))
		require.Equal(t, "3", string(savedTxs[2][0].Data))
	})

	t.Run("find error", func(t *testing.T) {
		ch := make(chan int)
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([]Tx, error) {
			defer func() { ch <- int(height) }()
			if height == 1 {
				return nil, nil
			}
			return nil, errors.New("boom")
		})
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs []Tx) error { return nil })

		collector := NewCollector(nopLog, finder, saver, time.Nanosecond)
		defer collector.Stop()
		go collector.Collect(context.Background())

		require.Equal(t, 1, <-ch)
		require.Equal(t, 2, <-ch)
		require.Equal(t, 2, <-ch) // assert height stops advancing
	})

	t.Run("save error", func(t *testing.T) {
		ch := make(chan int)
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([]Tx, error) {
			defer func() { ch <- int(height) }()
			return nil, nil
		})
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs []Tx) error {
			if height == 1 {
				return nil
			}
			return errors.New("boom")
		})

		collector := NewCollector(nopLog, finder, saver, time.Nanosecond)
		defer collector.Stop()
		go collector.Collect(context.Background())

		require.Equal(t, 1, <-ch)
		require.Equal(t, 2, <-ch)
		require.Equal(t, 2, <-ch) // assert height stops advancing
	})
}

func TestCollector_Stop(t *testing.T) {
	// Synchronization control to allow test to progress without a data race.
	// Begins locked, unlocks from the finder, and the test blocks trying to re-lock it.
	var foundMu sync.Mutex
	foundMu.Lock()

	// Ensures the finder only unlocks the mutex once.
	var foundOnce sync.Once

	finder := mockTxFinder(func(ctx context.Context, height uint64) ([]Tx, error) {
		foundOnce.Do(func() {
			foundMu.Unlock()
		})
		return nil, nil
	})
	saver := mockBlockSaver(func(ctx context.Context, height uint64, txs []Tx) error { return nil })

	c := NewCollector(zap.NewNop(), finder, saver, time.Millisecond)
	defer c.Stop() // Will be stopped explicitly in a few lines, but defer anyway for cleanup just in case.

	n := runtime.NumGoroutine()
	go c.Collect(context.Background())

	// Block until the finder was called at least once.
	foundMu.Lock()

	// At least one goroutine was created.
	require.Greater(t, runtime.NumGoroutine(), n)

	c.Stop()

	// require.Eventually would be nice here, but that starts its own goroutine,
	// which defeats the purpose of this test.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	require.Failf(t, "goroutine count did not drop after stopping collector", "want %d, got %d", n, runtime.NumGoroutine())
}
