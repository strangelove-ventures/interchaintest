package trace

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
    name TEXT NOT NULL CHECK ( length(name) >0 ),
    git_sha TEXT NOT NULL CHECK ( length(git_sha) >0 ),
    created_at TEXT NOT NULL,
    UNIQUE(name,created_at)
)`)
	if err != nil {
		return fmt.Errorf("create table test_case: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chain (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    identifier TEXT NOT NULL CHECK ( length(identifier) >0 ),
    test_id INTEGER,
    FOREIGN KEY(test_id) REFERENCES test_case(id) ON DELETE CASCADE,
    UNIQUE(identifier,test_id)
)`)
	if err != nil {
		return fmt.Errorf("create table chain: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS block (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    height INTEGER NOT NULL CHECK (length(height > 0)),
    chain_id INTEGER,
    FOREIGN KEY(chain_id) REFERENCES chain(id) ON DELETE CASCADE,
    UNIQUE(height,chain_id)
)`)
	if err != nil {
		return fmt.Errorf("create table block: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tx (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    json TEXT NOT NULL CHECK (length(json > 0)),
    block_id INTEGER,
    FOREIGN KEY(block_id) REFERENCES block(id) ON DELETE CASCADE
)`)
	if err != nil {
		return fmt.Errorf("create table tx: %w", err)
	}

	return nil
}
