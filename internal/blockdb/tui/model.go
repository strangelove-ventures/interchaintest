package tui

import (
	"time"

	"github.com/rivo/tview"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// Model encapsulates state that updates a view.
type Model struct {
	databasePath  string
	schemaVersion string
	schemaDate    time.Time
	testCases     []blockdb.TestCaseResult

	layout *tview.Flex
	stack  []tview.Primitive
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
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetBackgroundColor(backgroundColor).SetBorder(false)
	flex.AddItem(headerView(m), 0, 1, false)
	flex.AddItem(testCasesView(m), 0, 10, true)

	m.layout = flex

	return m
}

// RootView is a root view for a tview.Application.
func (m *Model) RootView() *tview.Flex {
	return m.layout
}
