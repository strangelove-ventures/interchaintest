package testutil

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// ChainHeighter fetches the current chain block height.
type ChainHeighter interface {
	Height(ctx context.Context) (int64, error)
}

// WaitForBlocks blocks until all chains reach a block height delta equal to or greater than the delta argument.
// If a ChainHeighter does not monotonically increase the height, this function may block program execution indefinitely.
func WaitForBlocks(ctx context.Context, delta int, chains ...ChainHeighter) error {
	if len(chains) == 0 {
		panic("missing chains")
	}
	eg, egCtx := errgroup.WithContext(ctx)
	for i := range chains {
		chain := chains[i]
		eg.Go(func() error {
			h := &height{Chain: chain}
			return h.WaitForDelta(egCtx, delta)
		})
	}
	return eg.Wait()
}

// WaitForBlocksUtil iterates from 0 to maxBlocks and calls fn function with the current iteration index as a parameter.
// If fn returns nil, the loop is terminated and the function returns nil.
// If fn returns an error and the loop has iterated over all maxBlocks without success, the error is returned.
func WaitForBlocksUtil(maxBlocks int, fn func(i int) error) error {
	for i := 0; i < maxBlocks; i++ {
		if err := fn(i); err == nil {
			break
		} else if i == maxBlocks-1 {
			return err
		}
	}
	return nil
}

// NodesInSync returns an error if the nodes are not in sync with the chain.
func NodesInSync(ctx context.Context, chain ChainHeighter, nodes []ChainHeighter) error {
	var chainHeight int64
	nodeHeights := make([]int64, len(nodes))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() (err error) {
		chainHeight, err = chain.Height(egCtx)
		return err
	})
	for i, n := range nodes {
		i := i
		n := n
		eg.Go(func() (err error) {
			nodeHeights[i], err = n.Height(egCtx)
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	for _, h := range nodeHeights {
		if h < chainHeight {
			return fmt.Errorf("Node is not yet in sync: %d < %d", h, chainHeight)
		}
	}
	// all nodes >= chainHeight
	return nil
}

// WaitForInSync blocks until all nodes have heights greater than or equal to the chain height.
func WaitForInSync(ctx context.Context, chain ChainHeighter, nodes ...ChainHeighter) error {
	if len(nodes) == 0 {
		panic("missing nodes")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := NodesInSync(ctx, chain, nodes); err != nil {
				continue
			}
			return nil
		}
	}
}

type height struct {
	Chain ChainHeighter

	starting int64
	current  int64
}

func (h *height) WaitForDelta(ctx context.Context, delta int) error {
	for h.delta() < delta {
		cur, err := h.Chain.Height(ctx)
		if err != nil {
			return err
		}
		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if cur == 0 {
			continue
		}
		h.update(cur)
	}
	return nil
}

func (h *height) delta() int {
	if h.starting == 0 {
		return 0
	}
	return int(h.current - h.starting)
}

func (h *height) update(height int64) {
	if h.starting == 0 {
		h.starting = height
	}
	h.current = height
}

// WaitForCondition periodically executes the given function fn based on the provided pollingInterval.
// The function fn should return true of the desired condition is met. If the function never returns true within the timeoutAfter
// period, or fn returns an error, the condition will not have been met.
func WaitForCondition(timeoutAfter, pollingInterval time.Duration, fn func() (bool, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutAfter)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed waiting for condition after %f seconds", timeoutAfter.Seconds())
		case <-time.After(pollingInterval):
			reachedCondition, err := fn()
			if err != nil {
				return fmt.Errorf("error occurred while waiting for condition: %s", err)
			}

			if reachedCondition {
				return nil
			}
		}
	}
}
