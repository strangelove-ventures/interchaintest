package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	background lipgloss.Color = "#000000"
	selected   lipgloss.Color = "#0096FF"
	border     lipgloss.Color = "#7393B3"
	text       lipgloss.Color = "#FFFFFF"

	hotPink lipgloss.Color = "#FF69B4"
)

var docStyle = lipgloss.NewStyle().
	Background(background)

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 03:04PM MST")
}
