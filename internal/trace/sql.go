package trace

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func ConnectDB(ctx context.Context, filePath string, maxConns int) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", filePath, err)
	}
	db.SetMaxOpenConns(maxConns)
	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db %s: %w", filePath, err)
	}
	return db, err
}
