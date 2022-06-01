package blockdb

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type TxFinder interface {
	FindTxs(ctx context.Context, height uint64) ([][]byte, error)
}

type BlockSaver interface {
	SaveBlock(ctx context.Context, height int, txs [][]byte) error
}

// Poller saves block transactions at regular intervals.
type Poller struct {
	finder TxFinder
	log    *zap.Logger
	rate   time.Duration
	saver  BlockSaver
}

// NewPoller creates a valid poller that polls every duration at rate.
func NewPoller(finder TxFinder, saver BlockSaver, rate time.Duration, log *zap.Logger) *Poller {
	return &Poller{
		finder: finder,
		log:    log,
		rate:   rate,
		saver:  saver,
	}
}

// Poll saves block transactions starting at height 1 and advancing by 1 height as long as there are
// no errors with finding or saving the transactions.
func (p *Poller) Poll(ctx context.Context) {
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

func (p *Poller) saveTxsForHeight(ctx context.Context, height uint64) error {
	txs, err := p.finder.FindTxs(ctx, height)
	if err != nil {
		return fmt.Errorf("find txs: %w", err)
	}
	err = p.saver.SaveBlock(ctx, int(height), txs)
	if err != nil {
		return fmt.Errorf("save block: %w", err)
	}
	return nil
}
