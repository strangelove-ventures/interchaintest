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

func TestChain_TraceBlock(t *testing.T) {
	var (
		ctx = context.Background()
		tx1 = []byte(`{"test":0}`)
		tx2 = []byte(`{"test":1}`)
	)

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.TraceBlock(ctx, 5, Txs{tx1, tx2})
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

		rows, err := db.Query(`SELECT json, block_id FROM tx`)
		require.NoError(t, err)
		defer rows.Close()
		var i int
		for rows.Next() {
			var (
				gotJSON    string
				gotBlockID int
			)
			require.NoError(t, rows.Scan(&gotJSON, &gotBlockID))
			require.Equal(t, 1, gotBlockID)
			require.JSONEq(t, fmt.Sprintf(`{"test":%d}`, i), gotJSON)
			i++
		}
		require.Equal(t, 2, i)
	})

	t.Run("idempotent", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.TraceBlock(ctx, 1, Txs{tx1})
		require.NoError(t, err)
		err = chain.TraceBlock(ctx, 1, Txs{tx1})
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

	t.Run("non-json tx", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)
		err := chain.TraceBlock(ctx, 1, Txs{[]byte(`not valid json`)})
		require.Error(t, err)
		require.Contains(t, err.Error(), "block 1: tx 0: malformed json")
	})
}
