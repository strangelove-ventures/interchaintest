package blockdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func emptyDB() *sql.DB {
	db, err := ConnectDB(context.Background(), ":memory:")
	if err != nil {
		panic(err)
	}
	return db
}

func migratedDB() *sql.DB {
	db := emptyDB()
	if err := Migrate(db, "test"); err != nil {
		panic(err)
	}
	return db
}

func TestConnectDB(t *testing.T) {
	file := filepath.Join(t.TempDir(), strconv.FormatInt(time.Now().UnixMilli(), 10), "test", t.Name()+".db")
	db, err := ConnectDB(context.Background(), file)

	require.NoError(t, err)
	require.NoError(t, db.Close())
}
