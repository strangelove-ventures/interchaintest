package blockdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuery_CurrentSchemaVersion(t *testing.T) {
	t.Parallel()

	db := emptyDB()
	defer db.Close()

	require.NoError(t, Migrate(db, "first-sha"))
	require.NoError(t, Migrate(db, "second-sha"))

	res, err := NewQuery(db).CurrentSchemaVersion(context.Background())

	require.NoError(t, err)
	require.Equal(t, "second-sha", res.GitSha)
	require.WithinDuration(t, res.CreatedAt, time.Now(), 10*time.Second)
}

func TestQuery_RecentTestCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test1", "sha1")
		require.NoError(t, err)
		c, err := tc.AddChain(ctx, "chain-b", "cosmos")
		require.NoError(t, err)
		require.NoError(t, c.SaveBlock(ctx, 10, [][]byte{[]byte("tx1"), []byte("tx2")}))
		require.NoError(t, c.SaveBlock(ctx, 11, [][]byte{[]byte("tx3")}))

		_, err = tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		_, err = CreateTestCase(ctx, db, "empty", "empty-test")
		require.NoError(t, err)

		results, err := NewQuery(db).RecentTestCases(ctx, 10)
		require.NoError(t, err)

		require.Len(t, results, 2)

		// No blocks or txs.
		got := results[0]
		require.EqualValues(t, 1, got.ID)
		require.Equal(t, "test1", got.Name)
		require.Equal(t, "sha1", got.GitSha)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 10*time.Second)
		require.Equal(t, "chain-a", got.ChainID)
		require.Equal(t, "cosmos", got.ChainType)
		require.EqualValues(t, 2, got.ChainPKey)
		require.Zero(t, got.ChainHeight.Int64)
		require.Zero(t, got.TxTotal.Int64)

		// With blocks and txs.
		got = results[1]
		require.EqualValues(t, 1, got.ID)
		require.Equal(t, "test1", got.Name)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 10*time.Second)
		require.Equal(t, "chain-b", got.ChainID)
		require.Equal(t, "cosmos", got.ChainType)
		require.EqualValues(t, 1, got.ChainPKey)
		require.EqualValues(t, 11, got.ChainHeight.Int64)
		require.EqualValues(t, 3, got.TxTotal.Int64)
	})

	t.Run("limit", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "1", "1")
		require.NoError(t, err)
		_, err = tc.AddChain(ctx, "chain1", "cosmos")
		require.NoError(t, err)
		_, err = tc.AddChain(ctx, "chain2", "cosmos")
		require.NoError(t, err)

		got, err := NewQuery(db).RecentTestCases(ctx, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})

	t.Run("no test cases", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		got, err := NewQuery(db).RecentTestCases(ctx, 1)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestQuery_BlocksWithTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test", "abc123")
		chain, err := tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		require.NoError(t, chain.SaveBlock(ctx, 2, [][]byte{[]byte(`1`)}))
		require.NoError(t, chain.SaveBlock(ctx, 4, [][]byte{[]byte(`2`), []byte(`3`)}))

		results, err := NewQuery(db).BlocksWithTx(ctx, chain.id)
		require.NoError(t, err)

		require.Len(t, results, 3)

		require.Equal(t, 2, results[0].Height)
		require.Equal(t, "1", string(results[0].Tx))

		require.Equal(t, 4, results[1].Height)
		require.Equal(t, "2", string(results[1].Tx))

		require.Equal(t, 4, results[2].Height)
		require.Equal(t, "3", string(results[2].Tx))
	})

	t.Run("no txs", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test", "abc123")
		chain, err := tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		results, err := NewQuery(db).BlocksWithTx(ctx, chain.id)
		require.NoError(t, err)

		require.Len(t, results, 0)
	})
}
