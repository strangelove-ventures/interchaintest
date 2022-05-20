package test

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// ChainHeighter fetches the current chain block height.
type ChainHeighter interface {
	Height(ctx context.Context) (uint64, error)
}

// WaitForBlocks blocks until all chains reach a block height delta equal to or greater than the delta argument.
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

type height struct {
	Chain ChainHeighter

	starting uint64
	current  uint64
}

func (h *height) WaitForDelta(ctx context.Context, delta int) error {
	for h.Delta() < delta {
		if err := h.UpdateOnce(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *height) UpdateOnce(ctx context.Context) error {
	cur, err := h.Chain.Height(ctx)
	if cur == 0 {
		panic("height cannot be zero")
	}
	if err != nil {
		return err
	}
	h.update(cur)
	return nil
}

func (h *height) Delta() int {
	if h.starting == 0 {
		return 0
	}
	return int(h.current - h.starting)
}

func (h *height) Current() uint64 {
	return h.current
}

func (h *height) update(height uint64) {
	if h.starting == 0 {
		h.starting = height
	}
	h.current = height
}
