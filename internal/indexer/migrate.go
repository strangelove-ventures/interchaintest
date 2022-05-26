package indexer

import "database/sql"

// TODO
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS test (
    id INTEGER NOT NULL PRIMARY KEY,
    name TEXT NOT NULL CHECK ( length(name) >0 ),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP 
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chain (
    id INTEGER NOT NULL PRIMARY KEY,
    identifier TEXT NOT NULL CHECK ( length(identifier) >0 ),
    test_id INTEGER,
    FOREIGN KEY(test_id) REFERENCES test(id) ON DELETE CASCADE
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS block (
    id INTEGER NOT NULL PRIMARY KEY,
    height INTEGER NOT NULL CHECK (length(height > 0)),
    chain_id INTEGER,
    FOREIGN KEY(chain_id) REFERENCES chain(id) ON DELETE CASCADE
)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tx (
    id INTEGER NOT NULL PRIMARY KEY,
    json TEXT NOT NULL CHECK (length(json > 0)),
    block_id INTEGER,
    FOREIGN KEY(block_id) REFERENCES block(id) ON DELETE CASCADE
)`)
	return err
}
