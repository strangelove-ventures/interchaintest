package trace

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func emptyDB() *sql.DB {
	db, err := ConnectDB(context.Background(), ":memory:", 3)
	if err != nil {
		panic(err)
	}
	return db
}

func migratedDB() *sql.DB {
	db := emptyDB()
	if err := Migrate(db); err != nil {
		panic(err)
	}
	return db
}

func TestConnectDB(t *testing.T) {
	f, err := os.CreateTemp("", t.Name())
	require.NoError(t, err)
	defer f.Close()
	defer os.RemoveAll(f.Name())

	db, err := ConnectDB(context.Background(), f.Name(), 10)
	require.NoError(t, err)
	require.NoError(t, db.Close())
}
