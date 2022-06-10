package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type keyBinding struct {
	Key  string // Single key or combination of keys.
	Help string // Very short help text describing the key's action.
}

type keyMap []keyBinding

var defaultHelpKeys = keyMap{
	{fmt.Sprintf("%c/k", tcell.RuneUArrow), "move up"},
	{fmt.Sprintf("%c/j", tcell.RuneDArrow), "move down"},
	{"esc", "go back"},
	{"ctl+c", "exit"},
}

func testCaseHelpKeys() keyMap {
	return append(defaultHelpKeys, keyBinding{"s", "cosmos summary"})
}
