package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTestCase_WithChain(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc := NewTestCase(db, "SomeTest")

		chain, err := tc.WithChain(context.Background(), "my-chain1")
		require.NoError(t, err)
		require.NotNil(t, chain)

		row := db.QueryRow(`SELECT name, created_at FROM test_case LIMIT 1`)
		var (
			gotName string
			gotTime string
		)
		err = row.Scan(&gotName, &gotTime)
		require.NoError(t, err)

		require.Equal(t, "SomeTest", gotName)

		ts, err := time.Parse(time.RFC3339, gotTime)
		require.NoError(t, err)
		require.WithinDuration(t, time.Now(), ts, 10*time.Second)

		row = db.QueryRow(`SELECT identifier, test_id FROM chain`)
		var (
			gotChain  string
			gotTestID int
		)
		err = row.Scan(&gotChain, &gotTestID)
		require.NoError(t, err)
		require.Equal(t, "my-chain1", gotChain)
		require.Equal(t, 1, gotTestID)
	})

	t.Run("idempotency", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc := NewTestCase(db, "ATest")

		_, err := tc.WithChain(context.Background(), "my-chain")
		require.NoError(t, err)
		_, err = tc.WithChain(context.Background(), "my-chain")
		require.NoError(t, err)

		row := db.QueryRow(`select count(*) from test_case`)
		var gotCount int
		err = row.Scan(&gotCount)
		require.NoError(t, err)
		require.Equal(t, 1, gotCount)

		row = db.QueryRow(`SELECT count(*) FROM chain`)
		err = row.Scan(&gotCount)
		require.NoError(t, err)
		require.Equal(t, 1, gotCount)
	})

	t.Run("error", func(t *testing.T) {
		db := emptyDB()
		defer db.Close()

		_, err := NewTestCase(db, "Failed").WithChain(context.Background(), "chain")
		require.Error(t, err)
	})
}
