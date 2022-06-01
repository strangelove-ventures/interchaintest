package blockdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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
	file := filepath.Join(os.TempDir(), strconv.FormatInt(time.Now().UnixMilli(), 10), "test", t.Name()+".db")
	defer os.RemoveAll(file)
	db, err := ConnectDB(context.Background(), file, 10)
	require.NoError(t, err)
	require.NoError(t, db.Close())
}
