package tui

import (
	"time"

	"github.com/rivo/tview"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=main

// main is the primary content for user interaction in the UI akin to html <main>.
type main int

const (
	testCasesMain main = iota
	cosmosSummaryMain
)

type mainStack []main

func (stack mainStack) Push(s main) []main { return append(stack, s) }
func (stack mainStack) Current() main      { return stack[len(stack)-1] }
func (stack mainStack) Pop() []main        { return stack[:len(stack)-1] }

// Model encapsulates state that updates a view.
type Model struct {
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
	databasePath string,
	schemaVersion string,
	schemaDate time.Time,
	testCases []blockdb.TestCaseResult,
) *Model {
	m := &Model{
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
