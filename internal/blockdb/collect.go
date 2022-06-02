package blockdb

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// TxFinder finds transactions given block at height.
type TxFinder interface {
	// FindTxs transactions should be in a human-readable format, preferably json.
	FindTxs(ctx context.Context, height uint64) ([][]byte, error)
}

// BlockSaver saves transactions for block at height.
type BlockSaver interface {
	SaveBlock(ctx context.Context, height uint64, txs [][]byte) error
}

// Collector saves block transactions at regular intervals.
type Collector struct {
	finder TxFinder
	log    *zap.Logger
	rate   time.Duration
	saver  BlockSaver
}

// NewCollector creates a valid Collector that polls every duration at rate.
// The rate should be less than the time it takes to produce a block.
// Typically, a rate that will collect a few times a second is sufficient such as 100-200ms.
func NewCollector(log *zap.Logger, finder TxFinder, saver BlockSaver, rate time.Duration) *Collector {
	return &Collector{
		finder: finder,
		log:    log,
		rate:   rate,
		saver:  saver,
	}
}

// Collect saves block transactions starting at height 1 and advancing by 1 height as long as there are
// no errors with finding or saving the transactions.
func (p *Collector) Collect(ctx context.Context) {
	tick := time.NewTicker(p.rate)
	defer tick.Stop()
	var height uint64 = 1
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := p.saveTxsForHeight(ctx, height); err != nil {
				p.log.Info("Failed to save transactions", zap.Error(err), zap.Uint64("height", height))
				continue
			}
			height++
		}
	}
}

func (p *Collector) saveTxsForHeight(ctx context.Context, height uint64) error {
	txs, err := p.finder.FindTxs(ctx, height)
	if err != nil {
		return fmt.Errorf("find txs: %w", err)
	}
	err = p.saver.SaveBlock(ctx, height, txs)
	if err != nil {
		return fmt.Errorf("save block: %w", err)
	}
	return nil
}
