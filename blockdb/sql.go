package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ConnectDB connects to the sqlite database at databasePath.
// Auto-creates directory path via MkdirAll.
// Pings database once to ensure connection.
// Pass :memory: as databasePath for in-memory database.
func ConnectDB(ctx context.Context, databasePath string) (*sql.DB, error) {
	if databasePath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(databasePath), 0755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", databasePath, err)
	}
	// Sqlite does not handle >1 open connections per process well,
	// otherwise "database is locked" errors frequently occur.
	db.SetMaxOpenConns(1)
	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db %s: %w", databasePath, err)
	}
	return db, err
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
