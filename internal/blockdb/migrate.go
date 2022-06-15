package blockdb

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
)

// Migrate migrates db in an idempotent manner.
// If an error is returned, it's acceptable to delete the database and start over.
// The basic ERD is as follows:
//  ┌────────────────────┐          ┌────────────────────┐         ┌────────────────────┐          ┌────────────────────┐
//  │                    │          │                    │         │                    │          │                    │
//  │                    │         ╱│                    │        ╱│                    │         ╱│                    │
//  │     Test Case      │───────┼──│       Chain        │───────○─│       Block        │────────○─│         Tx         │
//  │                    │         ╲│                    │        ╲│                    │         ╲│                    │
//  │                    │          │                    │         │                    │          │                    │
//  └────────────────────┘          └────────────────────┘         └────────────────────┘          └────────────────────┘
// The gitSha ensures we can trace back to the version of the codebase that produced the schema.
// Warning: Typical best practice wraps each migration step into its own transaction. For simplicity given
// this is an embedded database, we omit transactions.
func Migrate(db *sql.DB, gitSha string) error {
	// TODO(nix 05-27-2022): Appropriate indexes?
	_, err := db.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_version(
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL CHECK (length(created_at) > 0),
    git_sha TEXT NOT NULL CHECK (length(git_sha) > 0),
    UNIQUE(git_sha)
)`)
	if err != nil {
		return fmt.Errorf("create table schema_version: %w", err)
	}

	_, err = db.Exec(`INSERT INTO schema_version(created_at, git_sha) VALUES (?, ?) 
ON CONFLICT(git_sha) DO UPDATE SET git_sha=git_sha`, nowRFC3339(), gitSha)
	if err != nil {
		return fmt.Errorf("upsert schema_version with git sha %s: %w", gitSha, err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS test_case (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK ( length(name) > 0 ),
    git_sha TEXT NOT NULL CHECK ( length(git_sha) > 0 ),
    created_at TEXT NOT NULL CHECK (length(created_at) > 0),
    UNIQUE(name,created_at)
)`)
	if err != nil {
		return fmt.Errorf("create table test_case: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chain (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    chain_id TEXT NOT NULL CHECK ( length(chain_id) > 0 ),
    fk_test_id INTEGER,
    FOREIGN KEY(fk_test_id) REFERENCES test_case(id) ON DELETE CASCADE,
    UNIQUE(chain_id,fk_test_id)
)`)
	if err != nil {
		return fmt.Errorf("create table chain: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS block (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    height INTEGER NOT NULL CHECK (length(height > 0)),
    fk_chain_id INTEGER,
    created_at TEXT NOT NULL CHECK (length(created_at) > 0),
    FOREIGN KEY(fk_chain_id) REFERENCES chain(id) ON DELETE CASCADE,
    UNIQUE(height,fk_chain_id)
)`)
	if err != nil {
		return fmt.Errorf("create table block: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tx (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    data TEXT NOT NULL CHECK (length(data > 0)),
    fk_block_id INTEGER,
    FOREIGN KEY(fk_block_id) REFERENCES block(id) ON DELETE CASCADE
)`)
	if err != nil {
		return fmt.Errorf("create table tx: %w", err)
	}

	_, err = db.Exec(`ALTER TABLE chain ADD COLUMN chain_type TEXT NOT NULL check(length(chain_type) > 0) DEFAULT "unknown"`)
	if errIgnoreDuplicateColumn(err, "chain_type") != nil {
		return fmt.Errorf("alter table chain add chain_type: %w", err)
	}

	// Creating views should be last migration step.
	// Error already wrapped.
	return upsertViews(db)
}

// upsertViews should be idempotent by dropping/re-creating the view. The drop/re-create makes view authoring simpler
// in case table columns are altered, added, or dropped.
// Performance impact is negligible since views are essentially stored queries.
func upsertViews(db *sql.DB) error {
	// Drop and recreate views because it's performant and allows changing columns in earlier migration steps.

	_, err := db.Exec(`DROP VIEW IF EXISTS v_tx_flattened`)
	if err != nil {
		return fmt.Errorf("drop old v_tx_flattened view: %w", err)
	}

	_, err = db.Exec(`CREATE VIEW v_tx_flattened AS
SELECT
  test_case.id as test_case_id
  , test_case.created_at as test_case_created_at
  , test_case.name as test_case_name
  , chain.id as chain_kid
  , chain.chain_id as chain_id
  , chain.chain_type as chain_type
  , block.id as block_id
  , block.created_at as block_created_at
  , block.height as block_height
  , tx.id as tx_id
  , tx.data as tx
FROM tx
LEFT JOIN block ON tx.fk_block_id = block.id
LEFT JOIN chain ON block.fk_chain_id = chain.id
LEFT JOIN test_case ON chain.fk_test_id = test_case.id
`)
	if err != nil {
		return fmt.Errorf("create v_tx_flattened view: %w", err)
	}

	_, err = db.Exec(`DROP VIEW IF EXISTS v_cosmos_messages`)
	if err != nil {
		return fmt.Errorf("drop old v_cosmos_messages view: %w", err)
	}
	_, err = db.Exec(`CREATE VIEW v_cosmos_messages AS
SELECT
  test_case_id
  , test_case_name
  , chain_kid
  , chain_id
  , block_id
  , block_height
  , tx_id
  , key as msg_n -- message position within the tx
  , json_extract(value, "$.@type") as type
  , json_extract(value, "$.client_state.chain_id") as client_chain_id
  , json_extract(value, "$.client_id") as client_id
  , json_extract(value, "$.counterparty.client_id") as counterparty_client_id
  , json_extract(value, "$.connection_id") as conn_id
  , COALESCE(
      json_extract(value, "$.counterparty_connection_id"), -- ConnectionOpenAck
      json_extract(value, "$.counterparty.connection_id")  -- ConnectionOpenTry
    ) as counterparty_conn_id
  , COALESCE(
      json_extract(value, "$.port_id"),           -- ChannelOpen*
      json_extract(value, "$.source_port"),       -- MsgTransfer
      json_extract(value, "$.packet.source_port") -- MsgRecvPacket and MsgAcknowledgement (might be backwards)
    ) as port_id
  , COALESCE(
      json_extract(value, "$.channel.counterparty.port_id"), -- ChannelOpenTry
      json_extract(value, "$.packet.destination_port")       -- MsgRecvPacket and MsgAcknowledgement (might be backwards)
    ) as counterparty_port_id
  , COALESCE(
      json_extract(value, "$.channel_id"),           -- ChannelOpen*
      json_extract(value, "$.source_channel"),       -- MsgTransfer
      json_extract(value, "$.packet.source_channel") -- MsgRecvPacket and MsgAcknowledgement (might be backwards)
    ) as channel_id
  , COALESCE(
      json_extract(value, "$.counterparty_channel_id"),         -- ChannelOpenAck
      json_extract(value, "$.channel.counterparty.channel_id"), -- ChannelOpenTry
      json_extract(value, "$.packet.destination_channel")       -- MsgRecvPacket and MsgAcknowledgement (might be backwards)
    ) as counterparty_channel_id
  , value as raw
FROM v_tx_flattened, json_each(v_tx_flattened.tx, "$.body.messages")
`)
	if err != nil {
		return fmt.Errorf("create v_cosmos_messages view: %w", err)
	}

	_, err = db.Exec(`DROP VIEW IF EXISTS v_tx_agg`)
	if err != nil {
		return fmt.Errorf("drop old v_tx_agg view: %w", err)
	}

	_, err = db.Exec(`CREATE VIEW v_tx_agg AS
    SELECT 
       test_case.id AS test_case_id
     , test_case.created_at AS test_case_created_at
     , test_case.name AS test_case_name
     , test_case.git_sha AS test_case_git_sha
     , chain.id AS chain_kid
     , chain.chain_id AS chain_id
     , chain.chain_type AS chain_type
     , MAX(COALESCE(block.height, 0)) AS chain_height
     , COUNT(tx.data) AS tx_total
    FROM test_case
	LEFT JOIN chain ON chain.fk_test_id = test_case.id
	LEFT JOIN block ON block.fk_chain_id = chain.id
	LEFT JOIN tx ON tx.fk_block_id = block.id
	GROUP BY test_case.id, chain.id
`)
	if err != nil {
		return fmt.Errorf("create v_tx_agg view: %w", err)
	}

	return nil
}

func errIgnoreDuplicateColumn(err error, col string) error {
	var serr *sqlite.Error
	if errors.As(err, &serr) &&
		strings.Contains(serr.Error(), fmt.Sprintf("duplicate column name: %s", col)) {
		return nil
	}
	return err
}
