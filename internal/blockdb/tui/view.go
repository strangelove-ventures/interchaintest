package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func schemaVersionView(dbFilePath, gitSha string) string {
	bold := func(s string) string {
		return lipgloss.NewStyle().Foreground(hotPink).Bold(true).Render(s)
	}
	s := fmt.Sprintf("%s %s\n%s %s", bold("Database:"), dbFilePath, bold("Schema Version:"), gitSha)
	return lipgloss.NewStyle().
		Align(lipgloss.Left).
		Margin(0, 0, 1, 2).
		Render(s)
}

func newListModel(title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(selected).
		BorderForeground(selected)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy()

	l := list.New(nil, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(text)
	return l
}
