package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Update should be the argument for *(tview.Application).SetInputCapture.
// The Model potentially updates view state based on the event.
// Update must be called from the main goroutine. Otherwise, view updates will not render or cause data races.
// Per tview documentation, return nil to stop event propagation.
func (m *Model) Update(event *tcell.EventKey) *tcell.EventKey {
	defer m.updateHelp()

	if event.Key() == tcell.KeyESC {
		if len(m.stack) > 1 { // Stack must be at least 1, so we don't remove all main views.
			m.mainPagesView().RemovePage(m.stack.Current().String())
			m.stack = m.stack.Pop()
		}
	}

	switch event.Rune() {
	case 's':
		if m.stack.Current() == testCasesMain {
			m.stack = m.stack.Push(cosmosSummaryMain)
			m.mainPagesView().AddAndSwitchToPage(m.stack.Current().String(), tview.NewTextView().SetText("HI THERE"), true)
		}
	}

	return event
}

func (m *Model) mainPagesView() *tview.Pages {
	return m.layout.GetItem(1).(*tview.Pages)
}

func (m *Model) updateHelp() {
	header := m.layout.GetItem(0).(*tview.Flex) // header is a nested flex
	help := header.GetItem(0).(*helpView)
	keys := keyMap[m.stack.Current()]
	help.Update(keys)
}
