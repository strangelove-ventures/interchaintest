package blockdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"

	"golang.org/x/sync/singleflight"
)

// Chain tracks its blocks and a block's transactions.
type Chain struct {
	db     *sql.DB
	id     int64
	single singleflight.Group
}

type transactions [][]byte

func (txs transactions) hash() []byte {
	h := sha256.New()
	for _, tx := range txs {
		h.Write(tx)
	}
	return h.Sum(nil)
}

// SaveBlock tracks a block at height with its transactions.
// This method is idempotent and can be safely called multiple times with the same arguments.
// The txs should be human-readable.
func (chain *Chain) SaveBlock(ctx context.Context, height int, txs [][]byte) error {
	k := fmt.Sprintf("%d-%x", height, transactions(txs).hash())
	_, err, _ := chain.single.Do(k, func() (interface{}, error) {
		return nil, chain.saveBlock(ctx, height, txs)
	})
	return err
}

func (chain *Chain) saveBlock(ctx context.Context, height int, txs transactions) error {
	dbTx, err := chain.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dbTx.Rollback() }()

	res, err := dbTx.ExecContext(ctx, `INSERT OR REPLACE INTO block(height, chain_id) VALUES (?, ?)`, height, chain.id)
	if err != nil {
		return fmt.Errorf("insert into block: %w", err)
	}

	blockID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, tx := range txs {
		_, err = dbTx.ExecContext(ctx, `INSERT INTO tx(json, block_id) VALUES (?, ?)`, tx, blockID)
		if err != nil {
			return fmt.Errorf("insert into tx: %w", err)
		}
	}

	return dbTx.Commit()
}
