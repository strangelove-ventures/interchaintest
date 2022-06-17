package tui

import (
	"context"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Update should be the argument for *(tview.Application).SetInputCapture.
// The Model potentially updates view state based on the event.
// Update must be called from the main goroutine. Otherwise, view updates will not render or cause data races.
// Per tview documentation, return nil to stop event propagation.
func (m *Model) Update(ctx context.Context) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		oldMain := m.stack.Current()
		defer m.updateHelp(oldMain)

		switch {
		case event.Key() == tcell.KeyESC:
			if len(m.stack) > 1 { // Stack must be at least 1, so we don't remove all main content views.
				m.mainContentView().RemovePage(m.stack.Current().String())
				m.stack = m.stack.Pop()
				return nil
			}

		case event.Key() == tcell.KeyEnter && m.stack.Current() == testCasesMain:
			// Show tx detail.
			tc := m.testCases[m.selectedRow()]
			results, err := m.querySvc.Transactions(ctx, tc.ChainPKey)
			if err != nil {
				// TODO (nix - 6/14/22) Display error instead of panic.
				panic(err)
			}
			m.pushMainView(txDetailMain, newTxDetailView(tc.ChainID, results))
			return nil

		case event.Rune() == 'm' && m.stack.Current() == testCasesMain:
			// Show cosmos messages.
			tc := m.testCases[m.selectedRow()]
			results, err := m.querySvc.CosmosMessages(ctx, tc.ChainPKey)
			if err != nil {
				// TODO (nix - 6/14/22) Display error instead of panic.
				panic(err)
			}
			m.pushMainView(cosmosMessagesMain, cosmosMessagesView(tc, results))
			return nil

		case event.Rune() == '[' && m.stack.Current() == txDetailMain:
			goToPrevPage(m.txDetailView().Pages)
			return nil

		case event.Rune() == ']' && m.stack.Current() == txDetailMain:
			gotToNextPage(m.txDetailView().Pages)
			return nil

		case event.Rune() == '/' && m.stack.Current() == txDetailMain:
			m.txDetailView().ToggleSearch()
			return nil

		case event.Key() == tcell.KeyEnter && m.stack.Current() == txDetailMain:
			// Search tx detail.
			m.txDetailView().DoSearch()
			return nil
		}

		return event
	}
}

func (m *Model) updateHelp(oldMainContent mainContent) {
	// Prevent redrawing if nothing has changed.
	if oldMainContent == m.stack.Current() {
		return
	}
	help := m.layout.GetItem(0).(*tview.Flex).GetItem(0).(*helpView)
	help.Replace(keyMap[m.stack.Current()])
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

func (m *Model) txDetailView() *txDetailView {
	_, primitive := m.mainContentView().GetFrontPage()
	return primitive.(*txDetailView)
}

// gotToNextPage assumes a convention where the page name is equal to its index. e.g. "0", "1", "2", etc.
func gotToNextPage(pages *tview.Pages) {
	idxStr, _ := pages.GetFrontPage()
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		panic(err)
	}
	if idx == pages.GetPageCount()-1 {
		return
	}
	pages.SwitchToPage(strconv.Itoa(idx + 1))
}

// goToPrevPage assumes a convention where the page name is equal to its index. e.g. "0", "1", "2", etc.
func goToPrevPage(pages *tview.Pages) {
	idxStr, _ := pages.GetFrontPage()
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		panic(err)
	}
	if idx == 0 {
		return
	}
	pages.SwitchToPage(strconv.Itoa(idx - 1))
}
