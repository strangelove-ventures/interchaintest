package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func validChain(t *testing.T, db *sql.DB) *Chain {
	t.Helper()

	tc, err := CreateTestCase(context.Background(), db, "TestCase", "112233")
	require.NoError(t, err)
	c, err := tc.AddChain(context.Background(), "chain1")
	require.NoError(t, err)
	return c
}

func TestChain_SaveBlock(t *testing.T) {
	var (
		ctx = context.Background()
		tx1 = []byte(`{"test":0}`)
		tx2 = []byte(`{"test":1}`)
	)

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.SaveBlock(ctx, 5, transactions{tx1, tx2})
		require.NoError(t, err)

		row := db.QueryRow(`SELECT height, chain_id FROM block LIMIT 1`)
		var (
			gotHeight  int
			gotChainID int
		)
		err = row.Scan(&gotHeight, &gotChainID)
		require.NoError(t, err)

		require.Equal(t, 5, gotHeight)
		require.Equal(t, 1, gotChainID)

		rows, err := db.Query(`SELECT data, block_id FROM tx`)
		require.NoError(t, err)
		defer rows.Close()
		var i int
		for rows.Next() {
			var (
				gotData    string
				gotBlockID int
			)
			require.NoError(t, rows.Scan(&gotData, &gotBlockID))
			require.Equal(t, 1, gotBlockID)
			require.JSONEq(t, fmt.Sprintf(`{"test":%d}`, i), gotData)
			i++
		}
		require.Equal(t, 2, i)
	})

	t.Run("idempotent", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.SaveBlock(ctx, 1, transactions{tx1})
		require.NoError(t, err)
		err = chain.SaveBlock(ctx, 1, transactions{tx1})
		require.NoError(t, err)

		row := db.QueryRow(`SELECT count(*) FROM block`)
		var count int
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		row = db.QueryRow(`SELECT count(*) FROM tx`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("zero state", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.SaveBlock(ctx, 5, nil)
		require.NoError(t, err)

		row := db.QueryRow(`SELECT height FROM block LIMIT 1`)
		var gotHeight int
		err = row.Scan(&gotHeight)
		require.NoError(t, err)

		var count int
		row = db.QueryRow(`SELECT count(*) FROM tx`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Zero(t, count)
	})
}
