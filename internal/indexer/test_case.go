package indexer

import (
	"context"
	"database/sql"
	"time"
)

type TestCase struct {
	db      *sql.DB
	name    string
	created time.Time
}

func NewTestCase(db *sql.DB, testName string) *TestCase {
	return &TestCase{
		created: time.Now().UTC(),
		db:      db,
		name:    testName,
	}
}

// The identifier is a generalized unique id for the chain. In Cosmos, the chain id or chain name would be
// appropriate, for example.
func (tc *TestCase) WithChain(ctx context.Context, identifier string) (*Chain, error) {
	now := tc.created.Format(time.RFC3339)
	res, err := tc.db.ExecContext(ctx, `INSERT OR REPLACE INTO test_case(name, created_at) VALUES(?, ?)`, tc.name, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	_, err = tc.db.ExecContext(ctx, `INSERT INTO chain(identifier,test_id) VALUES(?, ?)`, identifier, id)
	return &Chain{
		identifier: identifier,
		db:         tc.db,
	}, nil
}
