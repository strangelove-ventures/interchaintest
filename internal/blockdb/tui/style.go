package tui

import "github.com/gdamore/tcell/v2"

const (
	backgroundColor = tcell.ColorBlack
	textColor       = tcell.ColorWhite
	errorTextColor  = tcell.ColorRed
)

var (
	textStyle = tcell.Style{}.Foreground(textColor)
)
