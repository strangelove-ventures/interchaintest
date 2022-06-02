package blockdb

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type mockTxFinder func(ctx context.Context, height uint64) ([][]byte, error)

func (f mockTxFinder) FindTxs(ctx context.Context, height uint64) ([][]byte, error) {
	return f(ctx, height)
}

type mockBlockSaver func(ctx context.Context, height uint64, txs [][]byte) error

func (f mockBlockSaver) SaveBlock(ctx context.Context, height uint64, txs [][]byte) error {
	return f(ctx, height, txs)
}

func TestCollector_Collect(t *testing.T) {
	nopLog := zap.NewNop()

	t.Run("happy path", func(t *testing.T) {
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([][]byte, error) {
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
				return [][]byte{[]byte(strconv.FormatUint(height, 10))}, nil
			}
		})

		var (
			currentHeight int64
			savedHeights  []int
			savedTxs      [][][]byte
		)
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs [][]byte) error {
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
		require.Equal(t, "1", string(savedTxs[0][0]))
		require.Equal(t, "2", string(savedTxs[1][0]))
		require.Equal(t, "3", string(savedTxs[2][0]))
	})

	t.Run("find error", func(t *testing.T) {
		ch := make(chan int)
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([][]byte, error) {
			defer func() { ch <- int(height) }()
			if height == 1 {
				return nil, nil
			}
			return nil, errors.New("boom")
		})
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs [][]byte) error { return nil })

		collector := NewCollector(nopLog, finder, saver, time.Nanosecond)
		go collector.Collect(context.Background())

		require.Equal(t, 1, <-ch)
		require.Equal(t, 2, <-ch)
		require.Equal(t, 2, <-ch) // assert height stops advancing
	})

	t.Run("save error", func(t *testing.T) {
		ch := make(chan int)
		finder := mockTxFinder(func(ctx context.Context, height uint64) ([][]byte, error) {
			defer func() { ch <- int(height) }()
			return nil, nil
		})
		saver := mockBlockSaver(func(ctx context.Context, height uint64, txs [][]byte) error {
			if height == 1 {
				return nil
			}
			return errors.New("boom")
		})

		collector := NewCollector(nopLog, finder, saver, time.Nanosecond)
		go collector.Collect(context.Background())

		require.Equal(t, 1, <-ch)
		require.Equal(t, 2, <-ch)
		require.Equal(t, 2, <-ch) // assert height stops advancing
	})
}
