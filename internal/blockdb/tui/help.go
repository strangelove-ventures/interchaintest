package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type keyBinding struct {
	Key  string // Single key or combination of keys.
	Help string // Very short help text describing the key's action.
}

var baseHelpKeys = []keyBinding{
	{"esc", "go back"},
	{"ctl+c", "exit"},
}

func bindingsWithBase(bindings ...[]keyBinding) []keyBinding {
	var all []keyBinding
	for i := range bindings {
		all = append(all, bindings[i]...)
	}
	return append(all, baseHelpKeys...)
}

var (
	tableNavKeys = []keyBinding{
		{fmt.Sprintf("%c/k", tcell.RuneUArrow), "move up"},
		{fmt.Sprintf("%c/j", tcell.RuneDArrow), "move down"},
	}
	textNavKeys = []keyBinding{
		{fmt.Sprintf("%c/k", tcell.RuneUArrow), "scroll up"},
		{fmt.Sprintf("%c/j", tcell.RuneDArrow), "scroll down"},
		{"g", "go to top"},
		{"shift+g", "go to bottom"},
		{"ctrl+b", "page up"},
		{"ctrl+f", "page down"},
	}

	keyMap = map[mainContent][]keyBinding{
		testCasesMain:      bindingsWithBase([]keyBinding{{"m", "cosmos messages"}, {"enter", "view txs"}}, tableNavKeys),
		cosmosMessagesMain: bindingsWithBase(tableNavKeys),
		txDetailMain: bindingsWithBase([]keyBinding{
			{"[", "previous tx"},
			{"]", "next tx"},
			{"/", "toggle search"},
			{"c", "copy all txs"},
		}, textNavKeys),
		errorModalMain: bindingsWithBase(nil),
	}
)

type helpView struct {
	*tview.Table
}

func newHelpView() *helpView {
	tbl := tview.NewTable().SetBorders(false)
	tbl.SetBorder(false)
	return &helpView{tbl}
}

// Replace serves as a hook to clear all keys and update the help table view with new keys.
func (view *helpView) Replace(keys []keyBinding) *helpView {
	view.Table.Clear()
	keyCell := func(s string) *tview.TableCell {
		return tview.NewTableCell("<" + s + ">").
			SetTextColor(tcell.ColorBlue)
	}
	textCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).
			SetStyle(textStyle.Attributes(tcell.AttrDim))
	}
	var (
		row int
		col int
	)
	for _, binding := range keys {
		// Only allow 6 help items per row or else help items will not be visible.
		if row > 0 && row%6 == 0 {
			row = 0
			col += 2
		}
		view.Table.SetCell(row, col, keyCell(binding.Key))
		view.Table.SetCell(row, col+1, textCell(binding.Help))
		row++
	}
	return view
}
