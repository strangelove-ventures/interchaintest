package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func schemaVersionView(gitSha string, createdAt time.Time) string {
	s := fmt.Sprintf("Schema Version:\n%s\n%s", gitSha, createdAt.Format(time.RFC822))
	return lipgloss.NewStyle().
		Align(lipgloss.Left).
		Background(background).
		Padding(1, 1, 1, 1).
		Render(s)
}
