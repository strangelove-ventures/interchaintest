package tui

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb"
	"github.com/stretchr/testify/require"
)

var (
	escKey   = tcell.NewEventKey(tcell.KeyESC, ' ', 0)
	enterKey = tcell.NewEventKey(tcell.KeyEnter, ' ', 0)
)

func runeKey(c rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, c, 0)
}

// draw is necessary for some of the below tests to get default behavior such as selecting the first available
// row in a *tview.Table.
func draw(view tview.Primitive) {
	view.Draw(tcell.NewSimulationScreen(""))
}

type mockQueryService struct {
	GotChainPkey int64
	Messages     []blockdb.CosmosMessageResult
	Txs          []blockdb.TxResult
	Err          error
}

func (m *mockQueryService) Transactions(ctx context.Context, chainPkey int64) ([]blockdb.TxResult, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotChainPkey = chainPkey
	return m.Txs, m.Err
}

func (m *mockQueryService) CosmosMessages(ctx context.Context, chainPkey int64) ([]blockdb.CosmosMessageResult, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotChainPkey = chainPkey
	return m.Messages, m.Err
}

func TestModel_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("go back", func(t *testing.T) {
		model := NewModel(&mockQueryService{}, "", "", time.Now(), nil)
		require.Equal(t, 1, model.mainContentView().GetPageCount())

		draw(model.RootView())

		update := model.Update(ctx)
		update(escKey)

		require.Equal(t, 1, model.mainContentView().GetPageCount())
	})

	t.Run("cosmos summary view", func(t *testing.T) {
		querySvc := &mockQueryService{
			Messages: []blockdb.CosmosMessageResult{
				{Height: 10},
				{Height: 11},
				{Height: 12},
			},
		}
		model := NewModel(querySvc, "", "", time.Now(), []blockdb.TestCaseResult{
			{ChainPKey: 5, ChainID: "my-chain1"},
			{ChainPKey: 6},
		})

		draw(model.RootView())

		update := model.Update(ctx)
		update(runeKey('m'))

		// By default, first row is selected in a rendered table.
		require.EqualValues(t, 5, querySvc.GotChainPkey)

		require.Equal(t, 2, model.mainContentView().GetPageCount())
		_, table := model.mainContentView().GetFrontPage()

		// 4 rows: 1 header + 3 blockdb.CosmosMessageResult
		require.Equal(t, 4, table.(*tview.Table).GetRowCount())
		require.Contains(t, table.(*tview.Table).GetTitle(), "my-chain1")
	})

	t.Run("tx detail", func(t *testing.T) {
		querySvc := &mockQueryService{
			Txs: []blockdb.TxResult{
				{Height: 12, Tx: []byte(`{"tx":1}`)},
				{Height: 13, Tx: []byte(`{"tx":2}`)},
				{Height: 14, Tx: []byte(`{"tx":3}`)},
			},
		}
		model := NewModel(querySvc, "", "", time.Now(), []blockdb.TestCaseResult{
			{ChainPKey: 5, ChainID: "my-chain1"},
			{ChainPKey: 6},
		})

		draw(model.RootView())

		update := model.Update(ctx)

		update(enterKey)

		// By default, first row is selected in a rendered table.
		require.EqualValues(t, 5, querySvc.GotChainPkey)

		require.Equal(t, 2, model.mainContentView().GetPageCount())
		txDetail := model.txDetailView()

		// Search and text view
		require.Equal(t, 2, txDetail.Flex.GetItemCount())

		require.Equal(t, 3, txDetail.Pages.GetPageCount())

		_, primitive := txDetail.Pages.GetFrontPage()
		textView := primitive.(*tview.TextView)

		require.Contains(t, textView.GetTitle(), "Tx 1 of 3")
		require.Contains(t, textView.GetTitle(), "my-chain1 @ Height 12")
		const wantFirstPage = `{
  "tx": 1
}`
		require.Equal(t, wantFirstPage, textView.GetText(true))

		// Move to the next page.
		update(runeKey(']'))

		_, primitive = txDetail.Pages.GetFrontPage()
		textView = primitive.(*tview.TextView)

		require.Contains(t, textView.GetTitle(), "Tx 2 of 3")
		require.Contains(t, textView.GetTitle(), "my-chain1 @ Height 13")
		const wantSecondPage = `{
  "tx": 2
}`
		require.Equal(t, wantSecondPage, textView.GetText(true))

		// Assert does not advance past last page.
		update(runeKey(']'))
		update(runeKey(']'))
		update(runeKey(']'))

		_, primitive = txDetail.Pages.GetFrontPage()
		textView = primitive.(*tview.TextView)

		require.Contains(t, textView.GetTitle(), "Tx 3 of 3")

		// Move back to the previous page. Assert does not retreat past first page.
		update(runeKey('['))
		update(runeKey('['))
		update(runeKey('['))
		update(runeKey('['))

		_, primitive = txDetail.Pages.GetFrontPage()
		textView = primitive.(*tview.TextView)

		require.Contains(t, textView.GetTitle(), "Tx 1 of 3")
	})

	t.Run("tx detail copy", func(t *testing.T) {
		querySvc := &mockQueryService{
			Txs: []blockdb.TxResult{
				{Height: 12, Tx: []byte(`{"tx":1}`)},
				{Height: 13, Tx: []byte(`{"tx":2}`)},
			},
		}
		model := NewModel(querySvc, "", "", time.Now(), []blockdb.TestCaseResult{
			{ChainPKey: 5, ChainID: "my-chain1"},
		})

		draw(model.RootView())

		var gotText string
		model.clipboard = func(text string) error {
			gotText = text
			return nil
		}

		update := model.Update(ctx)

		// Show tx detail.
		update(enterKey)
		// Now copy.
		update(runeKey('c'))

		require.NotEmpty(t, gotText)

		var gotTxs []any
		require.NoError(t, json.Unmarshal([]byte(gotText), &gotTxs))
		require.Len(t, gotTxs, 2)

		// Simulate clipboard failure.
		model.clipboard = func(string) error {
			return errors.New("boom")
		}

		update(runeKey('c'))

		_, primative := model.mainContentView().GetFrontPage()
		// TODO (nix - 6/22/22) Can't get text from a tview.Modal. We could use a tview.TextView but it does not render
		// properly with the nested flex views.
		require.IsType(t, &tview.Modal{}, primative.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1))
	})
}
