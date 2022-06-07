package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
)

var (
	quitKey = key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	)
	prevScreenKey = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "go to prev screen"),
	)
)

var listKeys = func() [][]key.Binding {
	defaults := list.DefaultKeyMap()
	return [][]key.Binding{
		{
			defaults.CursorUp,
			defaults.CursorDown,
			defaults.GoToStart,
			defaults.GoToEnd,
		},
		{
			prevScreenKey,
			quitKey,
		},
	}
}()

var viewportKeys = func() [][]key.Binding {
	defaults := viewport.DefaultKeyMap()
	return [][]key.Binding{
		{
			defaults.Up,
			defaults.Down,
			defaults.PageUp,
			defaults.PageDown,
			defaults.HalfPageUp,
			defaults.HalfPageDown,
		},
		{
			key.NewBinding(
				key.WithKeys("["),
				key.WithHelp("[", "go to prev block"),
			),
			key.NewBinding(
				key.WithKeys("]"),
				key.WithHelp("]", "go to next block"),
			),
			prevScreenKey,
			quitKey,
		},
	}
}()
