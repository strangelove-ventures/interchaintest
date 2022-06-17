package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validChain(t *testing.T, db *sql.DB) *Chain {
	t.Helper()

	tc, err := CreateTestCase(context.Background(), db, "TestCase", "112233")
	require.NoError(t, err)
	c, err := tc.AddChain(context.Background(), "chain1", "cosmos")
	require.NoError(t, err)
	return c
}

func TestChain_SaveBlock(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		tx1 = Tx{Data: []byte(`{"test":0}`)}
		tx2 = Tx{
			Data: []byte(`{"test":1}`),
			Events: []Event{
				{
					Type: "e1",
					Attributes: []EventAttribute{
						{Key: "k1", Value: "v1"},
					},
				},
				{
					Type: "e2",
					Attributes: []EventAttribute{
						{Key: "k2", Value: "v2"},
						{Key: "k3", Value: "v3"},
					},
				},
			},
		}
	)

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.SaveBlock(ctx, 5, []Tx{tx1, tx2})
		require.NoError(t, err)

		row := db.QueryRow(`SELECT height, fk_chain_id, created_at FROM block LIMIT 1`)
		var (
			gotHeight    int
			gotChainID   int
			gotCreatedAt string
		)
		err = row.Scan(&gotHeight, &gotChainID, &gotCreatedAt)
		require.NoError(t, err)

		require.Equal(t, 5, gotHeight)
		require.Equal(t, 1, gotChainID)

		ts, err := time.Parse(time.RFC3339, gotCreatedAt)
		require.NoError(t, err)
		require.WithinDuration(t, ts, time.Now(), 10*time.Second)

		rows, err := db.Query(`SELECT data, fk_block_id FROM tx`)
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

		rows, err = db.Query(`SELECT
tx.data, tendermint_event.type, key, value
FROM tendermint_event_attr
LEFT JOIN tendermint_event ON tendermint_event.id = fk_event_id
LEFT JOIN tx ON tx.id = tendermint_event.fk_tx_id
ORDER BY tendermint_event_attr.id`)
		require.NoError(t, err)
		defer rows.Close()
		for i = 0; rows.Next(); i++ {
			var gotData, gotType, gotKey, gotValue string
			require.NoError(t, rows.Scan(&gotData, &gotType, &gotKey, &gotValue))
			require.Equal(t, gotData, `{"test":1}`)
			switch i {
			case 0:
				require.Equal(t, gotType, "e1")
				require.Equal(t, gotKey, "k1")
				require.Equal(t, gotValue, "v1")
			case 1:
				require.Equal(t, gotType, "e2")
				require.Equal(t, gotKey, "k2")
				require.Equal(t, gotValue, "v2")
			case 2:
				require.Equal(t, gotType, "e2")
				require.Equal(t, gotKey, "k3")
				require.Equal(t, gotValue, "v3")
			default:
				t.Fatalf("expected 3 results, got i=%d", i)
			}
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		chain := validChain(t, db)

		err := chain.SaveBlock(ctx, 1, []Tx{tx2})
		require.NoError(t, err)
		err = chain.SaveBlock(ctx, 1, []Tx{tx2})
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

		row = db.QueryRow(`SELECT count(*) FROM tendermint_event`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		row = db.QueryRow(`SELECT count(*) FROM tendermint_event_attr`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 3, count)
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
		require.Equal(t, 5, gotHeight)

		var count int
		row = db.QueryRow(`SELECT count(*) FROM tx`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Zero(t, count)

		row = db.QueryRow(`SELECT count(*) FROM tendermint_event`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Zero(t, count)

		row = db.QueryRow(`SELECT count(*) FROM tendermint_event_attr`)
		err = row.Scan(&count)
		require.NoError(t, err)
		require.Zero(t, count)
	})
}
