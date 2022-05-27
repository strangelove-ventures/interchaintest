package trace

import (
	"context"
	"database/sql"
	"time"
)

type TestCase struct {
	db *sql.DB
	id int64
}

func CreateTestCase(ctx context.Context, db *sql.DB, testName string) (*TestCase, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `INSERT INTO test_case(name, created_at) VALUES(?, ?)`, testName, now)
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

// The identifier is a generalized unique id for the chain. In Cosmos, the chain id or chain name would be
// appropriate, for example.
func (tc *TestCase) AddChain(ctx context.Context, identifier string) (*Chain, error) {
	_, err := tc.db.ExecContext(ctx, `INSERT INTO chain(identifier,test_id) VALUES(?, ?)`, identifier, tc.id)
	if err != nil {
		return nil, err
	}
	return &Chain{
		identifier: identifier,
		db:         tc.db,
	}, nil
}
