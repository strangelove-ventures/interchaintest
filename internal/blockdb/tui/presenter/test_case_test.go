package presenter

import (
	"database/sql"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb"
	"github.com/stretchr/testify/require"
)

func TestTestCase(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		result := blockdb.TestCaseResult{
			ID:          321,
			Name:        "My Test",
			GitSha:      "sha1",
			CreatedAt:   time.Now(),
			ChainPKey:   789,
			ChainID:     "chain1",
			ChainType:   "cosmos",
			ChainHeight: sql.NullInt64{Int64: 77, Valid: true},
			TxTotal:     sql.NullInt64{Int64: 88, Valid: true},
		}

		pres := TestCase{result}
		require.Equal(t, "321", pres.ID())
		require.Equal(t, "My Test", pres.Name())
		require.Equal(t, "sha1", pres.GitSha())
		require.NotEmpty(t, pres.Date())
		require.Equal(t, "chain1", pres.ChainID())
		require.Equal(t, "77", pres.Height())
		require.Equal(t, "88", pres.TxTotal())
	})

	t.Run("zero state", func(t *testing.T) {
		var pres TestCase

		require.Empty(t, pres.Height())
		require.Empty(t, pres.TxTotal())
	})
}
