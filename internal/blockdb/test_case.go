package blockdb

import (
	"context"
	"database/sql"
)

// TestCase is a single test invocation.
type TestCase struct {
	db *sql.DB
	id int64
}

// CreateTestCase starts tracking new test case with testName.
func CreateTestCase(ctx context.Context, db *sql.DB, testName, gitSha string) (*TestCase, error) {
	res, err := db.ExecContext(ctx, `INSERT INTO test_case(name, created_at, git_sha) VALUES(?, ?, ?)`, testName, nowRFC3339(), gitSha)
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
// The chainID must be unique per test case. E.g. osmosis-1001, cosmos-1004
// The chainType denotes which ecosystem the chain belongs to. E.g. cosmos, penumbra, composable, etc.
func (tc *TestCase) AddChain(ctx context.Context, chainID, chainType string) (*Chain, error) {
	res, err := tc.db.ExecContext(ctx, `INSERT INTO chain(chain_id, chain_type, fk_test_id) VALUES(?, ?, ?)`, chainID, chainType, tc.id)
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
