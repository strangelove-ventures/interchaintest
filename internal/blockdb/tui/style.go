package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	selected lipgloss.Color = "#0096FF"
	hotPink  lipgloss.Color = "#FF69B4"
)

var (
	text = lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#000000"}
)

var docStyle = lipgloss.NewStyle()

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 03:04PM MST")
}
