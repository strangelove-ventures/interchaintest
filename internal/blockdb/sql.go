package blockdb

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// ConnectDB connects to the sqlite database at filePath with max connections set to maxConns.
// Pings database once to ensure connection.
// Pass :memory: as filePath for in-memory database.
func ConnectDB(ctx context.Context, filePath string, maxConns int) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", filePath, err)
	}
	db.SetMaxOpenConns(maxConns)
	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db %s: %w", filePath, err)
	}
	return db, err
}
