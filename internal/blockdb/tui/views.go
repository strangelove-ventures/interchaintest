package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func schemaVersionView(dbFilePath, gitSha string) string {
	bold := func(s string) string {
		return lipgloss.NewStyle().Bold(true).Render(s)
	}
	s := fmt.Sprintf("%s %s\n%s %s", bold("Database:"), dbFilePath, bold("Schema Version:"), gitSha)
	return lipgloss.NewStyle().
		Align(lipgloss.Left).
		Background(background).
		Padding(0, 1, 1, 1).
		Render(s)
}
