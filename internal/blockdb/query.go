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

// TestCaseResult is a combination of a single test case and single chain associated with the test case.
type TestCaseResult struct {
	ID          int64
	Name        string
	GitSha      string // Git commit that ran the test.
	CreatedAt   time.Time
	ChainPKey   int64  // chain primary key
	ChainID     string // E.g. osmosis-1001
	ChainType   string // E.g. cosmos, penumbra
	ChainHeight sql.NullInt64
	TxTotal     sql.NullInt64
}

// RecentTestCases returns aggregated data for each test case and chain combination.
func (q *Query) RecentTestCases(ctx context.Context, limit int) ([]TestCaseResult, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT 
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

type CosmosMessageResult struct {
	Height int64
	Index  int
	Type   string // URI for proto definition, e.g. /ibc.core.client.v1.MsgCreateClient

	ClientChainID sql.NullString

	ClientID             sql.NullString
	CounterpartyClientID sql.NullString

	ConnID             sql.NullString
	CounterpartyConnID sql.NullString

	PortID             sql.NullString
	CounterpartyPortID sql.NullString

	ChannelID             sql.NullString
	CounterpartyChannelID sql.NullString
}

// CosmosMessages returns a summary of Cosmos messages for the chainID. In Cosmos, a transaction may have 1 or more
// associated messages.
// chainPkey is the chain primary key "chain.id", not to be confused with the column "chain_id".
func (q *Query) CosmosMessages(ctx context.Context, chainPkey int64) ([]CosmosMessageResult, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT 
        block_height
        , msg_n -- message index or position within the tx
        , type
        , client_chain_id
        , client_id
        , counterparty_client_id
        , conn_id
        , counterparty_conn_id
        , port_id
        , counterparty_port_id
        , channel_id
        , counterparty_channel_id
    FROM v_cosmos_messages
    WHERE chain_kid = ?
    ORDER BY block_height ASC , msg_n ASC`, chainPkey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []CosmosMessageResult
	for rows.Next() {
		var res CosmosMessageResult
		if err = rows.Scan(
			&res.Height,
			&res.Index,
			&res.Type,
			&res.ClientChainID,
			&res.ClientID,
			&res.CounterpartyClientID,
			&res.ConnID,
			&res.CounterpartyConnID,
			&res.PortID,
			&res.CounterpartyPortID,
			&res.ChannelID,
			&res.CounterpartyChannelID,
		); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

type TxResult struct {
	Height int64
	Tx     []byte
}

// Transactions returns TxResults only for blocks with transactions present.
// chainPkey is the chain primary key "chain.id", not to be confused with the column "chain_id".
func (q *Query) Transactions(ctx context.Context, chainPkey int64) ([]TxResult, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT block.height, tx.data FROM tx 
    INNER JOIN block on tx.fk_block_id = block.id
    INNER JOIN chain on block.fk_chain_id = chain.id
    WHERE chain.id = ?
    ORDER BY block.height ASC, tx.id ASC`, chainPkey)
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
