package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	selectedColor = lipgloss.AdaptiveColor{Dark: "#0096FF", Light: "#1F51FF"} // blues
	textColor     = lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#000000"}

	docStyle = lipgloss.NewStyle()
)

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 03:04PM MST")
}
