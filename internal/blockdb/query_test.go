package blockdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuery_CurrentSchemaVersion(t *testing.T) {
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
	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		now := nowRFC3339()
		_, err := db.Exec(`INSERT INTO test_case (name, git_sha, created_at) VALUES 
			(?, ?, ?),
			(?, ?, ?),
			(?, ?, ?)`,
			"test1", "sha1", now,
			"test2", "sha2", now,
			"test3", "sha3", now)
		require.NoError(t, err)

		results, err := NewQuery(db).RecentTestCases(context.Background(), 10)

		require.NoError(t, err)
		require.Len(t, results, 3)

		require.Equal(t, "test3", results[0].Name)
		require.Equal(t, "sha3", results[0].GitSha)
		require.NotEmpty(t, results[0].CreatedAt)

		results, err = NewQuery(db).RecentTestCases(context.Background(), 1)

		require.NoError(t, err)
		require.Len(t, results, 1)
	})

	t.Run("no test cases", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		results, err := NewQuery(db).RecentTestCases(context.Background(), 1)

		require.NoError(t, err)
		require.Zero(t, results)
	})
}
