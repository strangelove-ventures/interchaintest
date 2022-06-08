package blockdb

import (
	"context"
	"database/sql"
	"time"
)

// Query is a service that queries the database.
type Query struct {
	db *sql.DB
}

func NewQuery(db *sql.DB) *Query {
	return &Query{db: db}
}

type SchemaVersionResult struct {
	GitSha string
	// Always set to user's local time zone.
	CreatedAt time.Time
}

// CurrentSchemaVersion returns the latest git sha and time that produced the sqlite schema.
func (q *Query) CurrentSchemaVersion(ctx context.Context) (SchemaVersionResult, error) {
	row := q.db.QueryRowContext(ctx, `SELECT git_sha, created_at FROM schema_version ORDER BY id DESC limit 1`)
	var (
		res      SchemaVersionResult
		createAt string
	)
	if err := row.Scan(&res.GitSha, &createAt); err != nil {
		return res, err
	}
	t, err := timeToLocal(createAt)
	if err != nil {
		return res, err
	}
	res.CreatedAt = t
	return res, nil
}

type TestCaseResult struct {
	ID        int64
	Name      string
	GitSha    string
	CreatedAt time.Time
	Chains    []string
}

type TxResult struct {
	Height int
	Tx     []byte
}

// BlocksWithTx returns TxResults only for blocks with transactions present.
// chainID is the chain primary key "chain.id", not to be confused with the column "chain_id".
func (q *Query) BlocksWithTx(ctx context.Context, chainID int64) ([]TxResult, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT block.height, tx.data FROM tx 
    INNER JOIN block on tx.fk_block_id = block.id
    INNER JOIN chain on block.fk_chain_id = chain.id
	WHERE chain.id = ?
	ORDER BY block.height ASC, tx.id ASC`, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TxResult
	for rows.Next() {
		var res TxResult
		if err := rows.Scan(&res.Height, &res.Tx); err != nil {
			return nil, err
		}
		results = append(results, res)
	}

	return results, nil
}
