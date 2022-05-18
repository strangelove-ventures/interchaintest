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
func WaitForBlocks(parent context.Context, delta int, chains ...ChainHeighter) error {
	if len(chains) == 0 {
		panic("missing chains")
	}
	eg, ctx := errgroup.WithContext(parent)
	for i := range chains {
		chain := chains[i]
		eg.Go(func() error {
			h := &height{}
			for h.Delta() < delta {
				cur, err := chain.Height(ctx)
				if err != nil {
					return err
				}
				h.Update(cur)
			}
			return nil
		})
	}
	return eg.Wait()
}

type height struct {
	Starting uint64
	Current  uint64
}

func (h *height) Delta() int {
	if h.Starting == 0 {
		return 0
	}
	return int(h.Current - h.Starting)
}

func (h *height) Update(height uint64) {
	if h.Starting == 0 {
		h.Starting = height
	}
	h.Current = height
}
