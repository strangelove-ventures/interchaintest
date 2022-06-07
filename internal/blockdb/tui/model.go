package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// QueryService queries a database and returns results.
type QueryService interface {
	Chains(ctx context.Context, testCaseID int64) ([]blockdb.ChainResult, error)
}

// Model is a tea.Model.
type Model struct {
	// See NewModel for rationale behind capturing context in a struct field.
	ctx       context.Context
	testCases []blockdb.TestCaseResult
	querySvc  QueryService

	currentScreen int

	headerView string

	testCaseList list.Model
	chainList    list.Model
	help         help.Model
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
		headerView:   schemaVersionView(dbFilePath, schemaGitSha),
		testCases:    testCases,
		testCaseList: tcList,
		chainList:    newListModel("Select Chain:"),
		help:         helpModel,
	}
}

// Init implements tea.Model. Currently, a nop.
func (m *Model) Init() tea.Cmd { return nil }

// View implements tea.Model.
func (m *Model) View() string {
	return docStyle.Render(
		lipgloss.JoinVertical(0,
			m.headerView,
			m.helpView(),
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
			panic(err)
		}
		m.chainList.SetItems(chainsToItems(chains))
		m.chainList, cmd = m.chainList.Update(msg)
	}

	return m, cmd
}

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
	m.help.Width = msg.Width
	h, v := docStyle.GetFrameSize()
	headerHeight := lipgloss.Height(m.headerView)
	footerHeight := lipgloss.Height(m.helpView())
	m.testCaseList.SetSize(msg.Width-h, msg.Height-v-headerHeight-footerHeight)
	m.chainList.SetSize(msg.Width-h, msg.Height-v-headerHeight-footerHeight)
}

func (m *Model) mainView() string {
	switch m.currentScreen {
	case screenTestCases:
		return m.testCaseList.View()
	case screenChains:
		return m.chainList.View()
	}
	panic(fmt.Errorf("unknown screen %d", m.currentScreen))
}

var helpStyle = lipgloss.NewStyle().Margin(0, 0, 1, 2)

func (m *Model) helpView() string {
	switch m.currentScreen {
	case screenTestCases, screenChains:
		return helpStyle.Render(m.help.FullHelpView(defaultKeys.FullHelp()))
	}
	panic(fmt.Errorf("unknown screen %d", m.currentScreen))
}

const (
	screenTestCases = iota
	screenChains
	screenBlocks
)
