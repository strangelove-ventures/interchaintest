package blockdb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/singleflight"
)

// Chain tracks its blocks and a block's transactions.
type Chain struct {
	db     *sql.DB
	id     int64
	single singleflight.Group
}

// Txs are transactions expected to be marshalled JSON.
type Txs [][]byte

func (txs Txs) prettyJSON() ([]string, error) {
	jsonTxs := make([]string, len(txs))
	buf := new(bytes.Buffer)
	for i, tx := range txs {
		if err := json.Indent(buf, tx, "", "  "); err != nil {
			return nil, fmt.Errorf("tx %d: malformed json: %w", i, err)
		}
		jsonTxs[i] = buf.String()
		buf.Reset()
	}
	return jsonTxs, nil
}

func (txs Txs) hash() []byte {
	h := sha256.New()
	for _, tx := range txs {
		h.Write(tx)
	}
	return h.Sum(nil)
}

// SaveBlock tracks a block at height with its transactions.
// This method is idempotent and can be safely called multiple times with the same arguments.
func (chain *Chain) SaveBlock(ctx context.Context, height int, txs Txs) error {
	k := fmt.Sprintf("%d-%x", height, txs.hash())
	_, err, _ := chain.single.Do(k, func() (interface{}, error) {
		return nil, chain.saveBlock(ctx, height, txs)
	})
	return err
}

func (chain *Chain) saveBlock(ctx context.Context, height int, txs Txs) error {
	// TODO(nix 05-27-2022): Presentation in the database layer is generally bad practice. However, the first pass
	// of this feature requires the user to make raw sql against the database. Therefore, to ease readability
	// we indent json here. If we have a presentation layer in the future, I suggest removing the json indent here
	// and let the presentation layer format appropriately.
	jsonTxs, err := txs.prettyJSON()
	if err != nil {
		return fmt.Errorf("block %d: %w", height, err)
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
