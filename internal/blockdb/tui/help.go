package tui

import "github.com/charmbracelet/bubbles/key"

type defaultKeyMap struct {
	GoToEnd    key.Binding
	GoToStart  key.Binding
	NextItem   key.Binding
	PrevItem   key.Binding
	PrevScreen key.Binding
	Quit       key.Binding
}

func (k defaultKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextItem, k.PrevItem, k.PrevScreen},
		{k.GoToStart, k.GoToEnd, k.Quit},
	}
}

var defaultKeys = defaultKeyMap{
	GoToStart: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("g/home", "go to start"),
	),
	GoToEnd: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("G/end", "go to end"),
	),
	NextItem: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "next item"),
	),
	PrevItem: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "previous item"),
	),
	PrevScreen: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "previous screen"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
