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
