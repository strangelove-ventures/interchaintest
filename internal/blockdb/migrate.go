package blockdb

import (
	"database/sql"
	"fmt"
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
func Migrate(db *sql.DB) error {
	// TODO(nix 05-27-2022): Appropriate indexes?
	_, err := db.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
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

	return nil
}
