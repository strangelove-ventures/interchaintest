package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) mainView() string {
	switch m.currentScreen {
	case screenTestCases:
		return m.testCaseList.View()
	case screenChains:
		return m.chainList.View()
	case screenBlocks:
		return m.blockDetailView()
	default:
		panic(fmt.Errorf("unknown screen %d", m.currentScreen))
	}
}

var helpStyle = lipgloss.NewStyle().Margin(0, 0, 1, 2)

func (m *Model) headerView() string {
	var groups [][]key.Binding
	switch m.currentScreen {
	case screenTestCases, screenChains:
		groups = listKeys
	case screenBlocks:
		groups = viewportKeys
	default:
		panic(fmt.Errorf("unknown screen %d", m.currentScreen))
	}
	return lipgloss.JoinHorizontal(0, helpStyle.Render(m.help.FullHelpView(groups)), m.schemaView)
}

func (m *Model) blockDetailView() string {
	var (
		tc    = m.testCases[m.testCaseList.Index()]
		chain = m.chainList.SelectedItem().(list.DefaultItem)
	)
	title := fmt.Sprintf("%s/%s", formatTime(tc.CreatedAt), chain.Title())
	title = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		MarginLeft(2).
		BorderForeground(borderColor).
		Foreground(textColor).
		Align(lipgloss.Center).
		Render(title)
	return title
}

func schemaVersionView(dbFilePath, gitSha string) string {
	bold := func(s string) string {
		return lipgloss.NewStyle().Bold(true).Render(s)
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
		Foreground(selectedColor).
		BorderForeground(selectedColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy()

	l := list.New(nil, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(textColor)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}
