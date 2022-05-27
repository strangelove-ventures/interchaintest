package trace

import (
	"database/sql"
)

// Chain tracks its blocks and a block's transactions.
type Chain struct {
	db *sql.DB
	id int64
}

// Txs are transactions expected to be marshalled JSON.
type Txs [][]byte

// TrackBlock tracks a block at height with its transactions.
// This method is idempotent and can be safely called multiple times with the same arguments.
func (chain *Chain) TrackBlock(height uint64, txs Txs) error {
	// TODO(nix 05-27-2022): Presentation in the database layer is generally bad practice. However, the first pass
	// of this feature requires the user to make raw sql against the database. Therefore, to ease readability
	// we indent json here. If we have a presentation layer in the future, I suggest removing the json indent here
	// and let the presentation layer format appropriately.
	return nil
}
