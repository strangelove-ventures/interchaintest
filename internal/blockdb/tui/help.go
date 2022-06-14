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

var defaultHelpKeys = []keyBinding{
	{fmt.Sprintf("%c/k", tcell.RuneUArrow), "move up"},
	{fmt.Sprintf("%c/j", tcell.RuneDArrow), "move down"},
	{"enter", "select row"},
	{"esc", "go back"},
	{"ctl+c", "exit"},
}

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
	for row, binding := range keys {
		view.Table.SetCell(row, 0, keyCell(binding.Key))
		view.Table.SetCell(row, 1, textCell(binding.Help))
	}
	return view
}
