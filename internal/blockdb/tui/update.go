package tui

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Update should be the argument for *(tview.Application).SetInputCapture.
// The Model potentially updates view state based on the event.
// Update must be called from the main goroutine. Otherwise, view updates will not render or cause data races.
// Per tview documentation, return nil to stop event propagation.
func (m *Model) Update(ctx context.Context) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		defer m.updateHelp()

		if event.Key() == tcell.KeyESC {
			if len(m.stack) > 1 { // Stack must be at least 1, so we don't remove all main content views.
				m.mainContentView().RemovePage(m.stack.Current().String())
				m.stack = m.stack.Pop()
			}
		}

		switch event.Rune() {
		case 's':
			if m.stack.Current() == testCasesMain {
				tc := m.testCases[m.selectedRow()]
				results, err := m.querySvc.CosmosMessages(ctx, tc.ChainPKey)
				if err != nil {
					// TODO (nix - 6/14/22) Display error instead of panic.
					panic(err)
				}
				m.pushMainView(cosmosSummaryMain, cosmosSummaryView(tc, results))
			}
		}

		return event
	}
}

func (m *Model) mainContentView() *tview.Pages {
	return m.layout.GetItem(1).(*tview.Pages)
}

func (m *Model) pushMainView(main mainContent, view tview.Primitive) {
	m.stack = m.stack.Push(main)
	m.mainContentView().AddAndSwitchToPage(main.String(), view, true)
}

func (m *Model) selectedRow() int {
	_, view := m.mainContentView().GetFrontPage()
	row, _ := view.(*tview.Table).GetSelection()
	// Offset by 1 to account for header row.
	return row - 1
}

func (m *Model) updateHelp() {
	header := m.layout.GetItem(0).(*tview.Flex) // header is a nested flex
	help := header.GetItem(0).(*helpView)
	keys := keyMap[m.stack.Current()]
	help.Update(keys)
}
