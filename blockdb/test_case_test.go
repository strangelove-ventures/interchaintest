package blockdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateTestCase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest", "abc123")
		require.NoError(t, err)
		require.NotNil(t, tc)

		row := db.QueryRow(`SELECT name, created_at, git_sha FROM test_case LIMIT 1`)
		var (
			gotName string
			gotTime string
			gotSha  string
		)
		err = row.Scan(&gotName, &gotTime, &gotSha)
		require.NoError(t, err)

		require.Equal(t, "SomeTest", gotName)
		require.Equal(t, "abc123", gotSha)

		ts, err := time.Parse(time.RFC3339, gotTime)
		require.NoError(t, err)
		require.WithinDuration(t, time.Now(), ts, 10*time.Second)
	})

	t.Run("errors", func(t *testing.T) {
		db := emptyDB()
		_, err := CreateTestCase(ctx, db, "fail", "")
		require.Error(t, err)
	})
}

func TestTestCase_AddChain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest", "abc")
		require.NoError(t, err)

		chain, err := tc.AddChain(ctx, "my-chain1", "penumbra")
		require.NoError(t, err)
		require.NotNil(t, chain)

		row := db.QueryRow(`SELECT chain_id, chain_type, fk_test_id, id FROM chain`)
		var (
			gotChainID    string
			gotChainType  string
			gotTestID     int
			gotPrimaryKey int64
		)
		err = row.Scan(&gotChainID, &gotChainType, &gotTestID, &gotPrimaryKey)
		require.NoError(t, err)
		require.Equal(t, "my-chain1", gotChainID)
		require.Equal(t, "penumbra", gotChainType)
		require.Equal(t, 1, gotTestID)
		require.EqualValues(t, 1, gotPrimaryKey)

		_, err = tc.AddChain(ctx, "my-chain2", "test")
		require.NoError(t, err)
	})

	t.Run("errors", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest", "abc")
		require.NoError(t, err)

		_, err = tc.AddChain(ctx, "my-chain", "cosmos")
		require.NoError(t, err)

		_, err = tc.AddChain(ctx, "my-chain", "cosmos")
		require.Error(t, err)
	})
}
