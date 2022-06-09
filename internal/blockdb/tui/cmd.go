package tui

import (
	"context"
	"fmt"

	"github.com/rivo/tview"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

func RunUI(ctx context.Context, databasePath string, gitSha string) error {
	db, err := blockdb.ConnectDB(ctx, databasePath)
	if err != nil {
		return fmt.Errorf("connect to database at %s: %w", databasePath, err)
	}
	defer db.Close()

	if err = blockdb.Migrate(db, gitSha); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	querySvc := blockdb.NewQuery(db)

	schemaInfo, err := querySvc.CurrentSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("query schema version: %w", err)
	}

	testCases, err := querySvc.RecentTestCases(ctx, 100)
	if err != nil {
		return fmt.Errorf("query recent test cases: %w", err)
	}
	if len(testCases) == 0 {
		return fmt.Errorf("no test cases found in database %s", databasePath)
	}

	app := tview.NewApplication()
	model := NewModel(schemaInfo.GitSha, schemaInfo.CreatedAt, testCases)

	return app.
		SetRoot(RootView(model), true).
		Run()
}
