package trace

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Chain tracks its blocks and a block's transactions.
type Chain struct {
	db *sql.DB
	id int64
}

// Txs are transactions expected to be marshalled JSON.
type Txs [][]byte

// TraceBlock tracks a block at height with its transactions.
// This method is idempotent and can be safely called multiple times with the same arguments.
func (chain *Chain) TraceBlock(ctx context.Context, height int, txs Txs) error {
	// TODO(nix 05-27-2022): Presentation in the database layer is generally bad practice. However, the first pass
	// of this feature requires the user to make raw sql against the database. Therefore, to ease readability
	// we indent json here. If we have a presentation layer in the future, I suggest removing the json indent here
	// and let the presentation layer format appropriately.
	jsonTxs := make([]string, len(txs))
	buf := new(bytes.Buffer)
	for i, tx := range txs {
		if err := json.Indent(buf, tx, "", "  "); err != nil {
			return fmt.Errorf("block %d: tx %d: malformed json: %w", height, i, err)
		}
		jsonTxs[i] = buf.String()
		buf.Reset()
	}

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
	for _, jtx := range jsonTxs {
		jtx := jtx
		_, err = dbTx.ExecContext(ctx, `INSERT INTO tx(json, block_id) VALUES (?, ?)`, jtx, blockID)
		if err != nil {
			return fmt.Errorf("insert into tx: %w", err)
		}
	}

	return dbTx.Commit()
}
