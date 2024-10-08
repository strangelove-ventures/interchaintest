package blockdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTxFlattenedView(t *testing.T) {
	t.Parallel()

	db := migratedDB()
	defer db.Close()

	ctx := context.Background()

	beforeTestCaseCreate := time.Now().UTC().Format(time.RFC3339)
	tc, err := CreateTestCase(ctx, db, "mytest", "abc123")
	require.NoError(t, err)

	chain, err := tc.AddChain(ctx, "chain1", "cosmos")
	require.NoError(t, err)

	beforeBlocksCreated := time.Now().UTC().Format(time.RFC3339)
	require.NoError(t, chain.SaveBlock(ctx, 1, []Tx{
		{Data: []byte("tx1.0")},
	}))
	require.NoError(t, chain.SaveBlock(ctx, 2, []Tx{
		{Data: []byte("tx2.0")},
		{Data: []byte("tx2.1")},
	}))
	afterBlocksCreated := time.Now().UTC().Format(time.RFC3339)

	var (
		tcID        int64
		tcCreatedAt string
		tcName      string

		chainKeyID int64
		chainID    string
		chainType  string

		blockID        int
		blockCreatedAt string
		blockHeight    int

		txID int
		tx   string
	)
	rows, err := db.Query(`SELECT
  test_case_id, test_case_created_at, test_case_name,
  chain_kid, chain_id, chain_type,
  block_id, block_created_at, block_height,
  tx_id, tx
FROM v_tx_flattened
ORDER BY test_case_id, chain_kid, block_id, tx_id
`)
	require.NoError(t, err)
	defer rows.Close()

	// Collect the first row and make assertions.
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(
		&tcID, &tcCreatedAt, &tcName,
		&chainKeyID, &chainID, &chainType,
		&blockID, &blockCreatedAt, &blockHeight,
		&txID, &tx,
	))

	require.Equal(t, tcID, tc.id)
	require.GreaterOrEqual(t, tcCreatedAt, beforeTestCaseCreate)
	require.LessOrEqual(t, tcCreatedAt, beforeBlocksCreated)
	require.Equal(t, "mytest", tcName)

	require.Equal(t, chainKeyID, chain.id)
	require.Equal(t, "chain1", chainID)
	require.Equal(t, "cosmos", chainType)

	require.GreaterOrEqual(t, blockCreatedAt, beforeBlocksCreated)
	require.LessOrEqual(t, blockCreatedAt, afterBlocksCreated)
	require.Equal(t, 1, blockHeight)

	require.Equal(t, "tx1.0", tx)

	// Save some state gathered from the first row.
	firstBlockCreatedAt := blockCreatedAt
	firstTxID := txID

	// Collect the second row and make assertions.
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(
		&tcID, &tcCreatedAt, &tcName,
		&chainKeyID, &chainID, &chainType,
		&blockID, &blockCreatedAt, &blockHeight,
		&txID, &tx,
	))

	// Same test case and chain.
	require.Equal(t, tcID, tc.id)
	require.Equal(t, chainKeyID, chain.id)

	// New block height.
	require.GreaterOrEqual(t, blockCreatedAt, firstBlockCreatedAt)
	require.LessOrEqual(t, blockCreatedAt, afterBlocksCreated)
	require.Equal(t, 2, blockHeight)

	// Next transaction.
	require.Greater(t, txID, firstTxID)
	require.Equal(t, "tx2.0", tx)

	secondTxID := txID

	// Third and final row.
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(
		&tcID, &tcCreatedAt, &tcName,
		&chainKeyID, &chainID, &chainType,
		&blockID, &blockCreatedAt, &blockHeight,
		&txID, &tx,
	))

	// Same test case and chain.
	require.Equal(t, tcID, tc.id)
	require.Equal(t, chainKeyID, chain.id)

	// Same block height.
	require.Equal(t, 2, blockHeight)

	// Next transaction.
	require.Greater(t, txID, secondTxID)
	require.Equal(t, "tx2.1", tx)

	// No more rows.
	require.False(t, rows.Next())
}

func TestTxAggView(t *testing.T) {
	// Nop. Tested as part of QueryService.
}
