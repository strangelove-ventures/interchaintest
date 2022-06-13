package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func headerView(m *Model) *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBorder(false)
	flex.SetBorderPadding(0, 0, 1, 1)

	help := newHelpView().Update(keyMap[testCasesMain])
	flex.AddItem(help, 0, 2, false)
	flex.AddItem(schemaVersionView(m), 0, 1, false)

	return flex
}

func schemaVersionView(m *Model) *tview.Table {
	tbl := tview.NewTable().SetBorders(false)
	tbl.SetBorder(false)

	titleCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).
			SetStyle(textStyle.Bold(true).Foreground(tcell.ColorDarkOrange))
	}
	tbl.SetCell(0, 0, titleCell("Database:"))
	tbl.SetCell(1, 0, titleCell("Schema Version:"))
	tbl.SetCell(2, 0, titleCell("Schema Date:"))

	valCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).SetStyle(textStyle)
	}
	tbl.SetCell(0, 1, valCell(m.databasePath))
	tbl.SetCell(1, 1, valCell(m.schemaVersion))
	tbl.SetCell(2, 1, valCell(formatTime(m.schemaDate)))

	return tbl
}

func detailTableView(title string, headers []string, rows [][]string) *tview.Table {
	if len(headers) == 0 {
		panic(errors.New("defaultTableView headers are required"))
	}
	if len(rows) == 0 {
		panic(errors.New("defaultTableView rows is required"))
	}
	tbl := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetSelectedStyle(tcell.Style{}.Foreground(backgroundColor).Background(textColor))
	tbl.
		SetBorder(true).
		SetBorderPadding(0, 0, 1, 1).
		SetBorderAttributes(tcell.AttrDim)

	tbl.SetTitle(title)

	headerCell := func(s string) *tview.TableCell {
		s = strings.ToUpper(s)
		return tview.NewTableCell(s).
			SetStyle(textStyle.Bold(true)).
			SetExpansion(1).
			SetSelectable(false)
	}

	for col, header := range headers {
		tbl.SetCell(0, col, headerCell(header))
	}

	contentCell := func(s string) *tview.TableCell {
		return tview.NewTableCell(s).SetStyle(textStyle).SetExpansion(1)
	}

	for i, row := range rows {
		rowPos := i + 1 // 1 offsets header row

		if len(row) != len(headers) {
			panic(fmt.Errorf("row %v column count %d must equal header count %d", row, len(row), len(headers)))
		}

		for col, content := range row {
			tbl.SetCell(rowPos, col, contentCell(content))
		}
	}
	return tbl
}

// testCasesView is the initial main content.
func testCasesView(m *Model) *tview.Table {
	headers := []string{
		"ID",
		"Date",
		"Name",
		"Git Sha",
		"Chain",
		"Height",
		"Tx Total",
	}

	rows := make([][]string, len(m.testCases))
	for i, tc := range m.testCases {
		pres := testCasePresenter{tc}
		rows[i] = []string{
			pres.ID(),
			pres.Date(),
			pres.Name(),
			pres.GitSha(),
			pres.ChainID(),
			pres.Height(),
			pres.TxTotal(),
		}
	}

	return detailTableView("Test Cases", headers, rows)
}
