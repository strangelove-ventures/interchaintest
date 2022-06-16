package blockdb

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Tx struct {
	// For Tendermint transactions, this should be encoded as JSON.
	// Otherwise, this should be a human-readable format if possible.
	Data []byte

	// Events associated with the transaction, if applicable.
	Events []Event
}

// Event is an alternative representation of tendermint/abci/types.Event,
// so that the blockdb package does not depend directly on tendermint.
type Event struct {
	Type       string
	Attributes []EventAttribute
}

type EventAttribute struct {
	Key, Value string
}

// TxFinder finds transactions given block at height.
type TxFinder interface {
	FindTxs(ctx context.Context, height uint64) ([]Tx, error)
}

// BlockSaver saves transactions for block at height.
type BlockSaver interface {
	SaveBlock(ctx context.Context, height uint64, txs []Tx) error
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
