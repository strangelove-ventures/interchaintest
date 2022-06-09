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
	testCases, err := querySvc.RecentTestCases(ctx, 100)
	if err != nil {
		return fmt.Errorf("query recent test cases: %w", err)
	}
	if len(testCases) == 0 {
		return fmt.Errorf("no test cases found in database %s", databasePath)
	}

	box := tview.NewBox().SetBorder(true).SetTitle("Hello, world!")
	app := tview.NewApplication()
	app.QueueUpdateDraw()
	return tview.NewApplication().
		SetFocus(box).
		SetRoot(box, true).
		Run()
}
