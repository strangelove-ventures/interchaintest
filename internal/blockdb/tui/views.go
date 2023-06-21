package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb"
	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb/tui/presenter"
)

func headerView(m *Model) *tview.Flex {
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBorder(false)
	flex.SetBorderPadding(0, 0, 1, 1)

	help := newHelpView().Replace(keyMap[testCasesMain])
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
	tbl.SetCell(2, 1, valCell(presenter.FormatTime(m.schemaDate)))

	return tbl
}

func detailTableView(title string, headers []string, rows [][]string) *tview.Table {
	if len(headers) == 0 {
		panic(errors.New("detailTableView headers are required"))
	}
	tbl := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetSelectedStyle(tcell.Style{}.Foreground(backgroundColor).Background(textColor)).
		SetFixed(1, 0)
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
		pres := presenter.TestCase{Result: tc}
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

func cosmosMessagesView(tc blockdb.TestCaseResult, msgs []blockdb.CosmosMessageResult) *tview.Table {
	headers := []string{
		"Height",
		"Index",
		"Type",
		"Client Chain",
		"Client",
		"Connection",
		"Channel:Port",
	}

	rows := make([][]string, len(msgs))
	for i, msg := range msgs {
		pres := presenter.CosmosMessage{Result: msg}
		rows[i] = []string{
			pres.Height(),
			pres.Index(),
			pres.Type(),
			pres.ClientChain(),
			pres.Clients(),
			pres.Connections(),
			pres.Channels(),
		}
	}

	title := fmt.Sprintf("%s [%s]", tc.ChainID, presenter.FormatTime(tc.CreatedAt))
	return detailTableView(title, headers, rows)
}

func errorModalView(err error) *tview.Flex {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Error: %v", err)).
		SetTextColor(errorTextColor).
		SetBackgroundColor(backgroundColor)

	// Flex centers the modal. See: https://github.com/rivo/tview/wiki/Modal
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modal, 0, 1, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)
}

const (
	searchActiveColor   = tcell.ColorPaleGreen
	searchInactiveColor = tcell.ColorBlue
)

// txDetailView is a very stateful view that could be refactored into a model.
// It allows a variety of interactions when viewing individual transactions.
type txDetailView struct {
	*tview.Flex

	chainID string

	Txs    []blockdb.TxResult
	Pages  *tview.Pages
	Search *tview.InputField
}

func newTxDetailView(chainID string, txs []blockdb.TxResult) *txDetailView {
	detail := &txDetailView{
		chainID: chainID,
		Txs:     txs,
	}

	detail.Pages = tview.NewPages()
	detail.replacePages("", "0")
	detail.Search = detail.buildSearchInput()

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetBorder(false)
	flex.AddItem(detail.Search, 3, 1, false)
	flex.AddItem(detail.Pages, 0, 9, true)

	detail.Flex = flex
	return detail
}

func (detail *txDetailView) ToggleSearch() {
	if detail.Search.HasFocus() {
		detail.deactivateSearch()
		return
	}
	detail.activateSearch()
}

func (detail *txDetailView) deactivateSearch() {
	detail.Search.SetBorderColor(searchInactiveColor)
	detail.Search.SetFieldTextColor(searchInactiveColor)
	detail.Search.SetTitleColor(searchInactiveColor)
	detail.Search.Blur()
	detail.Pages.Focus(nil)
}

func (detail *txDetailView) activateSearch() {
	detail.Search.SetBorderColor(searchActiveColor)
	detail.Search.SetFieldTextColor(searchActiveColor)
	detail.Search.SetTitleColor(searchActiveColor)
	detail.Search.Focus(nil)
	detail.Pages.Blur()
}

// DoSearch re-renders the text views with highlighted text.
func (detail *txDetailView) DoSearch() {
	detail.deactivateSearch()
	term := detail.Search.GetText()
	idx, _ := detail.Pages.GetFrontPage()
	detail.replacePages(term, idx)
}

// "pageIdx" is an integer string, e.g. "0", "1".
func (detail *txDetailView) replacePages(searchTerm, pageIdx string) {
	highlight := presenter.NewHighlight(searchTerm)
	for i, tx := range detail.Txs {
		idx := strconv.Itoa(i)
		detail.Pages.RemovePage(idx)

		pres := presenter.Tx{Result: tx}
		text, regions := highlight.Text(pres.Data())
		textView := tview.NewTextView().
			SetText(text).
			SetTextColor(textColor).
			SetWrap(true).
			SetWordWrap(true).
			SetTextAlign(tview.AlignLeft).
			SetScrollable(true)

		// Support highlighting text.
		textView.SetDynamicColors(true).SetRegions(true)
		textView.Highlight(regions...).ScrollToHighlight()

		textView.SetBorder(true).
			SetBorderPadding(0, 0, 1, 1).
			SetBorderAttributes(tcell.AttrDim)

		textView.SetTitle(fmt.Sprintf("%s @ Height %d [Tx %d of %d]", detail.chainID, tx.Height, i+1, len(detail.Txs)))

		detail.Pages.AddPage(idx, textView, true, false)
	}

	detail.Pages.SwitchToPage(pageIdx)
}

func (*txDetailView) buildSearchInput() *tview.InputField {
	input := tview.NewInputField().
		SetFieldTextColor(searchInactiveColor).
		SetFieldBackgroundColor(backgroundColor)

	input.SetTitle("Search").
		SetTitleColor(searchInactiveColor).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetBorderAttributes(tcell.AttrDim).
		SetBorderColor(searchInactiveColor)
	return input
}
