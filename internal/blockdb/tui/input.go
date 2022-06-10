package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

//type Drawer interface {
//	Draw() *tview.Application
//}

// HandleInput should be the argument for *(tview.Application).SetInputCapture.
// The Model potentially updates view state based on the event.
// HandleInput must be called from the main goroutine. Otherwise, view updates will not render.
// Per tview documentation, return nil to stop event propagation.
func (m *Model) HandleInput(app *tview.Application) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 's':
			m.updateHelpKeys()
		}
		return event
	}
}

func (m *Model) updateHelpKeys() {
	header := m.layout.GetItem(0).(*tview.Flex)
	header.RemoveItem(header.GetItem(0))
}
