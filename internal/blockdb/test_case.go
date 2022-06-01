package blockdb

import (
	"context"
	"database/sql"
	"time"
)

// TestCase is a single test invocation.
type TestCase struct {
	db *sql.DB
	id int64
}

// CreateTestCase starts tracking new test case with testName.
func CreateTestCase(ctx context.Context, db *sql.DB, testName, gitSha string) (*TestCase, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `INSERT INTO test_case(name, created_at, git_sha) VALUES(?, ?, ?)`, testName, now, gitSha)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &TestCase{
		db: db,
		id: id,
	}, nil
}

// AddChain tracks and attaches a chain to the test case.
// The identifier is a generalized id or name for the chain. In Cosmos, the chain id or chain name would be
// appropriate, for example.
// The position ensures uniqueness when testing multiple of the same chain, e.g. gaia <-> gaia.
func (tc *TestCase) AddChain(ctx context.Context, position int, identifier string) (*Chain, error) {
	res, err := tc.db.ExecContext(ctx, `INSERT INTO chain(position, identifier, test_id) VALUES(?, ?, ?)`, position, identifier, tc.id)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Chain{
		id: id,
		db: tc.db,
	}, nil
}
