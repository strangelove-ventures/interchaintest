package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// keyMap inner slice be exactly 2 elements.
// First element is the key. Second element is the help text.
type keyMap [][]string

var defaultHelpKeys = [][]string{
	{fmt.Sprintf("%c/k", tcell.RuneUArrow), "move up"},
	{fmt.Sprintf("%c/j", tcell.RuneDArrow), "move down"},
	{"esc", "go back"},
	{"ctl+c", "exit"},
}
