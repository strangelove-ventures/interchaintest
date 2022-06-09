package blockdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		db := emptyDB()
		defer db.Close()

		const gitSha = "abc123"
		err := Migrate(db, gitSha)
		require.NoError(t, err)

		// idempotent
		err = Migrate(db, gitSha)
		require.NoError(t, err)

		row := db.QueryRow(`select count(*) from schema_version`)
		var count int
		err = row.Scan(&count)

		require.NoError(t, err)
		require.Equal(t, 1, count)

		err = Migrate(db, "new-sha")
		require.NoError(t, err)

		row = db.QueryRow(`select count(*) from schema_version`)
		err = row.Scan(&count)

		require.NoError(t, err)
		require.Equal(t, 2, count)

		row = db.QueryRow(`select git_sha from schema_version order by id desc limit 1`)
		var gotSha string
		err = row.Scan(&gotSha)

		require.NoError(t, err)
		require.Equal(t, "new-sha", gotSha)
	})

	t.Run("idempotent", func(t *testing.T) {
		// Start with an existing schema.
		db := migratedDB()
		defer db.Close()

		// Migrate against the existing schema.
		err := Migrate(db, "abc123")
		require.NoError(t, err)
	})
}
