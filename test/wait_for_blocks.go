package test

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// ChainHeighter fetches the current chain block height.
type ChainHeighter interface {
	Height(ctx context.Context) (uint64, error)
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

// WaitForInSync blocks until all nodes have heights greater than or equal to the chain height.
func WaitForInSync(ctx context.Context, chain ChainHeighter, nodes ...ChainHeighter) error {
	if len(nodes) == 0 {
		panic("missing nodes")
	}
InSyncLoop:
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("timed out waiting for node(s) to be in sync: %w", ctx.Err())
		}
		var chainHeight uint64
		nodeHeights := make([]uint64, len(nodes))
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
				fmt.Printf("Node height %d is less than chain height %d\n", h, chainHeight)
				continue InSyncLoop
			}
		}
		// all nodes >= chainHeight
		return nil
	}
}

type height struct {
	Chain ChainHeighter

	starting uint64
	current  uint64
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

func (h *height) update(height uint64) {
	if h.starting == 0 {
		h.starting = height
	}
	h.current = height
}
