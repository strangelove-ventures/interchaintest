package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"

	"golang.org/x/sync/singleflight"
)

// Chain tracks its blocks and a block's transactions.
type Chain struct {
	db     *sql.DB
	id     int64
	single singleflight.Group
}

type transactions []Tx

func (txs transactions) Hash() []byte {
	h := fnv.New32()
	for _, tx := range txs {
		h.Write(tx.Data)
	}
	return h.Sum(nil)
}

// SaveBlock tracks a block at height with its transactions.
// This method is idempotent and can be safely called multiple times with the same arguments.
// The txs should be human-readable.
func (chain *Chain) SaveBlock(ctx context.Context, height uint64, txs []Tx) error {
	k := fmt.Sprintf("%d-%x", height, transactions(txs).Hash())
	_, err, _ := chain.single.Do(k, func() (any, error) {
		return nil, chain.saveBlock(ctx, height, txs)
	})
	return err
}

func (chain *Chain) saveBlock(ctx context.Context, height uint64, txs transactions) error {
	dbTx, err := chain.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dbTx.Rollback() }()

	res, err := dbTx.ExecContext(ctx, `INSERT OR REPLACE INTO block(height, fk_chain_id, created_at) VALUES (?, ?, ?)`, height, chain.id, nowRFC3339())
	if err != nil {
		return fmt.Errorf("insert into block: %w", err)
	}

	blockID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, tx := range txs {
		txRes, err := dbTx.ExecContext(ctx, `INSERT INTO tx(data, fk_block_id) VALUES (?, ?)`, string(tx.Data), blockID)
		if err != nil {
			return fmt.Errorf("insert into tx: %w", err)
		}
		txID, err := txRes.LastInsertId()
		if err != nil {
			return err
		}

		for _, e := range tx.Events {
			eventRes, err := dbTx.ExecContext(ctx, `INSERT INTO tendermint_event(type, fk_tx_id) VALUES (?, ?)`, e.Type, txID)
			if err != nil {
				return fmt.Errorf("insert into tendermint_event: %w", err)
			}

			eventID, err := eventRes.LastInsertId()
			if err != nil {
				return err
			}

			for _, attr := range e.Attributes {
				_, err := dbTx.ExecContext(ctx, `INSERT INTO tendermint_event_attr(key, value, fk_event_id) VALUES (?, ?, ?)`, attr.Key, attr.Value, eventID)
				if err != nil {
					return fmt.Errorf("insert into tendermint_event_attr: %w", err)
				}
			}
		}
	}

	return dbTx.Commit()
}
