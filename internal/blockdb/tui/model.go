package tui

import (
	"context"
	"errors"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// QueryService queries a database and returns results.
type QueryService interface {
	BlocksWithTx(ctx context.Context, chainID int64) ([]blockdb.TxResult, error)
	Chains(ctx context.Context, testCaseID int64) ([]blockdb.ChainResult, error)
}

// Model is a tea.Model.
type Model struct {
	// See NewModel for rationale behind capturing context in a struct field.
	ctx      context.Context
	querySvc QueryService

	testCases []blockdb.TestCaseResult
	chains    []blockdb.ChainResult
	txs       []blockdb.TxResult

	currentScreen int

	schemaView string

	testCaseList list.Model
	chainList    list.Model
	help         help.Model
	blockModel   viewport.Model
}

// NewModel returns a valid *Model.
// The args ctx, querySvc, dbFilePath, and schemaGitSha are required or this function panics.
// We capture ctx into a struct field which is not idiomatic. However, the tea.Model interface does not allow
// passing a context. Therefore, we must capture it in the constructor.
func NewModel(
	ctx context.Context,
	querySvc QueryService,
	dbFilePath string,
	schemaGitSha string,
	testCases []blockdb.TestCaseResult) *Model {
	if querySvc == nil {
		panic(errors.New("querySvc missing"))
	}
	if dbFilePath == "" {
		panic(errors.New("dbFilePath missing"))
	}
	if schemaGitSha == "" {
		panic(errors.New("schemaGitSha missing"))
	}

	tcList := newListModel("Select Test Case:")
	tcList.SetItems(testCasesToItems(testCases))

	helpModel := help.New()

	return &Model{
		ctx:          ctx,
		querySvc:     querySvc,
		schemaView:   schemaVersionView(dbFilePath, schemaGitSha),
		testCases:    testCases,
		testCaseList: tcList,
		chainList:    newListModel("Select Chain:"),
		help:         helpModel,
		blockModel:   viewport.New(20, 20),
	}
}

// Init implements tea.Model. Currently, a nop.
func (m *Model) Init() tea.Cmd { return nil }

// View implements tea.Model.
func (m *Model) View() string {
	return docStyle.Render(
		lipgloss.JoinVertical(0,
			m.headerView(),
			m.mainView(),
		),
	)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			m.incrementScreen()
		case "esc":
			m.decrementScreen()
		}
	case tea.WindowSizeMsg:
		m.updateLayout(msg)
	}

	var cmd tea.Cmd
	switch m.currentScreen {

	case screenTestCases:
		m.testCaseList, cmd = m.testCaseList.Update(msg)

	case screenChains:
		tc := m.testCases[m.testCaseList.Index()]
		chains, err := m.querySvc.Chains(m.ctx, tc.ID)
		if err != nil {
			// If error here, something is wrong with the db.
			panic(err)
		}
		m.chains = chains
		m.chainList.SetItems(chainsToItems(chains))
		m.chainList, cmd = m.chainList.Update(msg)

	case screenBlocks:
		txs, err := m.querySvc.BlocksWithTx(m.ctx, m.chains[m.chainList.Index()].ID)
		if err != nil {
			// If error here, something is wrong with the db.
			panic(err)
		}
		m.txs = txs
		m.blockModel.SetContent(txPresenter(txs[0].Tx).String())
		m.blockModel, cmd = m.blockModel.Update(msg)
	}

	return m, cmd
}

// screen stack
const (
	screenTestCases = iota
	screenChains
	screenBlocks
)

func (m *Model) incrementScreen() {
	if m.currentScreen == screenBlocks {
		return
	}
	m.currentScreen++
}

func (m *Model) decrementScreen() {
	if m.currentScreen == 0 {
		return
	}
	m.currentScreen--
}

func (m *Model) updateLayout(msg tea.WindowSizeMsg) {
	// List views requires special layout handling.
	m.help.Width = msg.Width
	x, y := docStyle.GetFrameSize()
	adjustedW := msg.Width - x
	adjustedH := msg.Height - y - lipgloss.Height(m.headerView())
	m.testCaseList.SetSize(adjustedW, adjustedH)
	m.chainList.SetSize(adjustedW, adjustedH)
	m.blockModel.Width = adjustedW
	m.blockModel.Height = adjustedH
	m.blockModel.YPosition = lipgloss.Height(m.headerView()) + 1
}
