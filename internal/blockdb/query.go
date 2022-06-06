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
}

func (q *Query) RecentTestCases(ctx context.Context, limit int) ([]TestCaseResult, error) {
	rows, err := q.db.Query(`SELECT id, name, git_sha, created_at FROM test_case ORDER BY ID DESC LIMIT ?`, limit)
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
		if err := rows.Scan(&res.ID, &res.Name, &res.GitSha, &createdAt); err != nil {
			return nil, err
		}
		t, err := timeToLocal(createdAt)
		if err != nil {
			return nil, err
		}
		res.CreatedAt = t
		results = append(results, res)
	}

	return results, nil
}

type ChainResult struct {
	ID      int64
	ChainID string
	Height  int
}

func (q *Query) Chains(ctx context.Context, testCaseID int64) ([]ChainResult, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT chain.id, chain.chain_id, MAX(COALESCE(block.height, 0)) FROM chain 
    LEFT JOIN block ON block.fk_chain_id = chain.id
    WHERE chain.fk_test_id = ?
    GROUP BY chain.id
    ORDER BY chain.chain_id ASC`, testCaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ChainResult
	for rows.Next() {
		var res ChainResult
		if err := rows.Scan(&res.ID, &res.ChainID, &res.Height); err != nil {
			return nil, err
		}
		results = append(results, res)
	}

	return results, nil
}
