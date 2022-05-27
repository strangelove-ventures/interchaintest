package trace

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateTestCase(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest")
		require.NoError(t, err)
		require.NotNil(t, tc)

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
	})

	t.Run("errors", func(t *testing.T) {
		db := emptyDB()
		_, err := CreateTestCase(ctx, db, "fail")
		require.Error(t, err)
	})
}

func TestTestCase_WithChain(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest")
		require.NoError(t, err)

		chain, err := tc.AddChain(ctx, "my-chain1")
		require.NoError(t, err)
		require.NotNil(t, chain)

		row := db.QueryRow(`SELECT identifier, test_id FROM chain`)
		var (
			gotChain  string
			gotTestID int
		)
		err = row.Scan(&gotChain, &gotTestID)
		require.NoError(t, err)
		require.Equal(t, "my-chain1", gotChain)
		require.Equal(t, 1, gotTestID)

		_, err = tc.AddChain(ctx, "my-chain2")
		require.NoError(t, err)
	})

	t.Run("errors", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "SomeTest")
		require.NoError(t, err)

		_, err = tc.AddChain(ctx, "my-chain")
		require.NoError(t, err)

		_, err = tc.AddChain(ctx, "my-chain")
		require.Error(t, err)
	})
}
