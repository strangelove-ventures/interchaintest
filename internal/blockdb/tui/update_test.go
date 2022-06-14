package tui

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
	"github.com/stretchr/testify/require"
)

type mockQueryService struct {
	GotChainID int64
	Messages   []blockdb.CosmosMessageResult
	Err        error
}

func (m *mockQueryService) CosmosMessages(ctx context.Context, chainID int64) ([]blockdb.CosmosMessageResult, error) {
	if ctx == nil {
		panic("nil context")
	}
	m.GotChainID = chainID
	return m.Messages, m.Err
}

func TestModel_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("go back", func(t *testing.T) {
		model := NewModel(&mockQueryService{}, "", "", time.Now(), nil)
		require.Equal(t, 1, model.mainContentView().GetPageCount())

		update := model.Update(ctx)
		update(tcell.NewEventKey(tcell.KeyESC, ' ', 0))

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

		// Must draw screen first to get default behavior.
		model.RootView().Draw(tcell.NewSimulationScreen(""))

		update := model.Update(ctx)
		update(tcell.NewEventKey(tcell.KeyRune, 's', 0))

		// By default, first row is selected in a rendered table.
		require.EqualValues(t, 5, querySvc.GotChainID)

		require.Equal(t, 2, model.mainContentView().GetPageCount())
		_, table := model.mainContentView().GetFrontPage()

		// 4 rows: 1 header + 3 blockdb.CosmosMessageResult
		require.Equal(t, 4, table.(*tview.Table).GetRowCount())
		require.Contains(t, table.(*tview.Table).GetTitle(), "my-chain1")

		update(tcell.NewEventKey(tcell.KeyRune, 's', 0))
		// Assert page count unchanged with duplicate key presses.
		require.Equal(t, 2, model.mainContentView().GetPageCount())
	})
}
