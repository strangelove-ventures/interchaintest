package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Query is a service that queries the database.
type Query struct {
	db *sql.DB
}

func NewQuery(db *sql.DB) *Query {
	return &Query{db: db}
}

func timeToLocal(timeStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("time.Parse RFC3339: %w", err)
	}
	return t.In(time.Local), nil
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
		return res, fmt.Errorf("parse createdAt: %w", err)
	}
	res.CreatedAt = t
	return res, nil
}

type TestCaseResult struct {
	ID          int64
	Name        string
	GitSha      string // Git commit that ran the test.
	CreatedAt   time.Time
	ChainPKey   int64  // Integer primary key.
	ChainID     string // E.g. osmosis-1001
	ChainType   string // E.g. cosmos, penumbra
	ChainHeight sql.NullInt64
	TxTotal     sql.NullInt64
}

func (q *Query) RecentTestCases(ctx context.Context, limit int) ([]TestCaseResult, error) {
	rows, err := q.db.QueryContext(ctx, `
	SELECT 
    	test_case_id, test_case_created_at, test_case_name, test_case_git_sha, chain_kid, chain_id, chain_type, chain_height, tx_total
	FROM v_tx_agg 
	WHERE chain_kid IS NOT NULL
	ORDER BY test_case_id DESC, chain_id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []TestCaseResult
	for rows.Next() {
		var (
			res       TestCaseResult
			createdAt string
		)
		if err = rows.Scan(
			&res.ID,
			&createdAt,
			&res.Name,
			&res.GitSha,
			&res.ChainPKey,
			&res.ChainID,
			&res.ChainType,
			&res.ChainHeight,
			&res.TxTotal,
		); err != nil {
			return nil, err
		}
		t, err := timeToLocal(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse createdAt: %w", err)
		}
		res.CreatedAt = t
		results = append(results, res)
	}
	return results, nil
}

type TxResult struct {
	Height int
	Tx     []byte
}
