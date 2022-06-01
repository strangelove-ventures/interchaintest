package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ConnectDB connects to the sqlite database at databaseFile.
// Pings database once to ensure connection.
// Creates directory path via MkdirAll.
// Pass :memory: as databaseFile for in-memory database.
func ConnectDB(ctx context.Context, databaseFile string) (*sql.DB, error) {
	if databaseFile != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databaseFile), 0755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", databaseFile)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", databaseFile, err)
	}
	db.SetMaxOpenConns(1)
	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db %s: %w", databaseFile, err)
	}
	return db, err
}
