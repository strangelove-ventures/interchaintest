package tui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func headerView(m *Model) *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBorder(false)
	flex.SetBorderPadding(0, 0, 1, 1)

	flex.AddItem(helpView(defaultHelpKeys), 0, 3, false)
	flex.AddItem(schemaVersionView(m.schemaVersion, m.schemaDate), 0, 1, false)
	return flex
}

func schemaVersionView(schemaVersion string, schemaDate time.Time) *tview.Table {
	tbl := tview.NewTable().SetBorders(false)
	tbl.SetBorder(false)

	titleCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).
			SetStyle(textStyle.Bold(true).Foreground(tcell.ColorDarkOrange))
	}
	tbl.SetCell(0, 0, titleCell("Schema Version:"))
	tbl.SetCell(1, 0, titleCell("Schema Date:"))

	valCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).SetStyle(textStyle)
	}
	tbl.SetCell(0, 1, valCell(schemaVersion))
	tbl.SetCell(1, 1, valCell(formatTime(schemaDate)))

	return tbl
}

func helpView(keys [][]string) *tview.Table {
	tbl := tview.NewTable().SetBorders(false)
	tbl.SetBorder(false)

	keyCell := func(s string) *tview.TableCell {
		return tview.NewTableCell("<" + s + ">").
			SetTextColor(tcell.ColorBlue)
	}
	textCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).
			SetStyle(textStyle.Attributes(tcell.AttrDim))
	}
	for row, keymap := range keys {
		tbl.SetCell(row, 0, keyCell(keymap[0]))
		tbl.SetCell(row, 1, textCell(keymap[1]))
	}
	return tbl
}

func testCasesView(m *Model) *tview.Table {
	tbl := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetSelectedStyle(tcell.Style{}.Foreground(backgroundColor).Background(textColor))
	tbl.
		SetBorder(true).
		SetBorderPadding(0, 0, 1, 1).
		SetBorderAttributes(tcell.AttrDim)
	tbl.SetTitle("Test Cases")

	headerCell := func(s string) *tview.TableCell {
		s = strings.ToUpper(s)
		return tview.NewTableCell(s).
			SetStyle(textStyle.Bold(true)).
			SetExpansion(1).
			SetSelectable(false)
	}
	for col, title := range []string{
		"Date",
		"Name",
		"Git Sha",
		"Chain",
		"Height",
		"Tx Total",
	} {
		tbl.SetCell(0, col, headerCell(title))
	}

	contentCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).SetStyle(textStyle).SetExpansion(1)
	}

	for i, tc := range m.testCases {
		row := i + 1 // 1 offsets header row
		pres := testCasePresenter{tc}
		tbl.SetCell(row, 0, contentCell(pres.Date()))
		tbl.SetCell(row, 1, contentCell(pres.Name()))
		tbl.SetCell(row, 2, contentCell(pres.GitSha()))
		tbl.SetCell(row, 3, contentCell(pres.ChainID()))
		tbl.SetCell(row, 4, contentCell(pres.Height()).SetAlign(tview.AlignRight))
		tbl.SetCell(row, 5, contentCell(pres.TxTotal()).SetAlign(tview.AlignRight))
	}
	return tbl
}
