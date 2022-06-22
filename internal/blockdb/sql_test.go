package blockdb

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func emptyDB() *sql.DB {
	db, err := ConnectDB(context.Background(), ":memory:")
	if err != nil {
		panic(err)
	}
	return db
}

func migratedDB() *sql.DB {
	db := emptyDB()
	if err := Migrate(db, "test"); err != nil {
		panic(err)
	}
	return db
}

func TestConnectDB(t *testing.T) {
	file := filepath.Join(t.TempDir(), strconv.FormatInt(time.Now().UnixMilli(), 10), "test", t.Name()+".db")
	db, err := ConnectDB(context.Background(), file)

	require.NoError(t, err)
	require.NoError(t, db.Close())
}

// Test that multiple writers and readers against the same underlying file
// do not fail due to a "database is locked" error.
func TestDB_Concurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to short mode")
	}

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbPath := filepath.Join(t.TempDir(), "concurrent.db")

	// Block all writers until this channel is closed.
	beginWrites := make(chan struct{})

	const nWriters = 4
	const nTestCases = 500
	const nQueriers = 2
	const sha = "abc123"

	// Dedicated errgroup for the writers,
	// and a shared context so all fail if one fails.
	egWrites, egCtx := errgroup.WithContext(ctx)
	for i := 0; i < nWriters; i++ {
		i := i

		// Connecting to the database in the main goroutine
		// because concurrently connecting to the same database
		// causes a data race inside sqlite.
		db, err := ConnectDB(ctx, dbPath)
		require.NoErrorf(t, err, "failed to connect to db for writer %d: %v", i, err)
		defer db.Close()
		require.NoError(t, Migrate(db, sha))

		egWrites.Go(func() error {
			// Block until this channel is closed.
			<-beginWrites

			for j := 0; j < nTestCases; j++ {
				tc, err := CreateTestCase(egCtx, db, fmt.Sprintf("test-%d-%d", i, j), sha)
				if err != nil {
					return fmt.Errorf("writer %d failed to create test case %d/%d: %w", i, j+1, nTestCases, err)
				}
				time.Sleep(time.Millisecond)

				_, err = tc.AddChain(egCtx, fmt.Sprintf("chain-%d-%d", i, j), "cosmos")
				if err != nil {
					return fmt.Errorf("writer %d failed to add chain to test case %d/%d: %w", i, j+1, nTestCases, err)
				}
				time.Sleep(time.Millisecond)
			}

			return nil
		})
	}

	// Separate errgroup for the queriers.
	var egQueries errgroup.Group
	for i := 0; i < nQueriers; i++ {
		i := i

		db, err := ConnectDB(ctx, dbPath)
		require.NoErrorf(t, err, "failed to connect to db for querier %d: %v", i, err)
		defer db.Close()
		require.NoError(t, Migrate(db, sha))

		egQueries.Go(func() error {
			// No need to synchronize here; just begin querying.
			q := NewQuery(db)

			for {
				if ctx.Err() != nil {
					// Context was canceled; querying is finished.
					return nil
				}

				// Deliberately using context.Background() here so that
				// canceling the writers does not cause an "interrupted" error
				// when querying the recent test cases.
				// (This must be an sqlite implementation detail, as that error
				// is distinct from context.Canceled.)
				_, err := q.RecentTestCases(context.Background(), nTestCases*nWriters)
				if err != nil {
					return fmt.Errorf("error in querier %d retrieving test cases: %w", i, err)
				}
			}
		})
	}

	// Signal that writes can begin, then wait for them to finish.
	close(beginWrites)
	require.NoError(t, egWrites.Wait())

	// Signal that queries should stop, then wait for them to finish.
	cancel()
	require.NoError(t, egQueries.Wait())

	// Final assertions against written number of test cases.
	db, err := ConnectDB(context.Background(), dbPath)
	require.NoErrorf(t, err, "failed to connect to db for final assertion")
	defer db.Close()

	tcs, err := NewQuery(db).RecentTestCases(context.Background(), nTestCases*nWriters)
	require.NoError(t, err, "failed to collect recent test cases")

	require.Len(t, tcs, nTestCases*nWriters, "incorrect count on final written test cases")
}
