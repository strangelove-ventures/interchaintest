package trace

import "database/sql"

// TODO: indexes?
// Migrate migrates db in an idempotent manner.
// If an error is returned, it's acceptable to delete the database file and start over.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS test_case (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK ( length(name) >0 ),
    created_at TEXT NOT NULL,
    UNIQUE(name,created_at)
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chain (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    identifier TEXT NOT NULL CHECK ( length(identifier) >0 ),
    test_id INTEGER,
    FOREIGN KEY(test_id) REFERENCES test_case(id) ON DELETE CASCADE,
    UNIQUE(identifier,test_id)
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS block (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    height INTEGER NOT NULL CHECK (length(height > 0)),
    chain_id INTEGER,
    FOREIGN KEY(chain_id) REFERENCES chain(id) ON DELETE CASCADE
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tx (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    json TEXT NOT NULL CHECK (length(json > 0)),
    block_id INTEGER,
    FOREIGN KEY(block_id) REFERENCES block(id) ON DELETE CASCADE
)`)
	return err
}
