package tui

import (
	"context"
	"time"

	"github.com/rivo/tview"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=mainContent

// mainContent is the primary content for user interaction in the UI akin to html <main>.
type mainContent int

const (
	testCasesMain mainContent = iota
	cosmosMessagesMain
	txDetailMain
)

type mainStack []mainContent

func (stack mainStack) Push(s mainContent) []mainContent { return append(stack, s) }
func (stack mainStack) Current() mainContent             { return stack[len(stack)-1] }
func (stack mainStack) Pop() []mainContent               { return stack[:len(stack)-1] }

// QueryService fetches data from a database.
type QueryService interface {
	CosmosMessages(ctx context.Context, chainPkey int64) ([]blockdb.CosmosMessageResult, error)
	Transactions(ctx context.Context, chainPkey int64) ([]blockdb.TxResult, error)
}

// Model encapsulates state that updates a view.
type Model struct {
	querySvc      QueryService
	databasePath  string
	schemaVersion string
	schemaDate    time.Time
	testCases     []blockdb.TestCaseResult

	layout *tview.Flex

	// stack keeps tracks of primary content pushed and popped
	stack mainStack
}

// NewModel returns a valid *Model.
func NewModel(
	querySvc QueryService,
	databasePath string,
	schemaVersion string,
	schemaDate time.Time,
	testCases []blockdb.TestCaseResult,
) *Model {
	m := &Model{
		querySvc:      querySvc,
		databasePath:  databasePath,
		schemaVersion: schemaVersion,
		schemaDate:    schemaDate,
		testCases:     testCases,
		stack:         mainStack{testCasesMain},
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetBackgroundColor(backgroundColor).SetBorder(false)
	// Setting fixed size keeps the header height stable, so it will show all help keys.
	flex.AddItem(headerView(m), 6, 1, false)

	// The primary view is a page view to act like a stack where we can push and pop views.
	// Flex and grid views do not allow a "stack-like" behavior.
	pages := tview.NewPages()
	pages.AddAndSwitchToPage(m.stack[0].String(), testCasesView(m), true)
	flex.AddItem(pages, 0, 10, true)

	m.layout = flex

	return m
}

// RootView is a root view for a tview.Application.
func (m *Model) RootView() *tview.Flex {
	return m.layout
}
