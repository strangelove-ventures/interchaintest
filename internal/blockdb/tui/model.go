package tui

import (
	"context"
	"errors"

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
	// See NewModel godoc for rationale behind capturing context in a struct field.
	ctx          context.Context
	dbFilePath   string
	schemaGitSha string
	testCases    []blockdb.TestCaseResult

	testCaseList list.Model
	chainList    list.Model
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
		panic(errors.New("querySvc required"))
	}
	if dbFilePath == "" {
		panic(errors.New("dbFilePath required"))
	}
	if schemaGitSha == "" {
		panic(errors.New("schemaGitSha required"))
	}
	return &Model{
		ctx:          ctx,
		dbFilePath:   dbFilePath,
		schemaGitSha: schemaGitSha,
		testCases:    testCases,
		testCaseList: newListModel("Select Test Case", testCasesToItems(testCases)),
	}
}

// Init implements tea.Model.
// Init panics if any exported field is not set.
func (m *Model) Init() tea.Cmd {
	m.testCaseList = newListModel("Select Test Case", testCasesToItems(m.testCases))
	return nil
}

// View implements tea.Model.
func (m *Model) View() string {
	return docStyle.Render(
		lipgloss.JoinVertical(0,
			schemaVersionView(m.dbFilePath, m.schemaGitSha), m.testCaseList.View(),
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
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.testCaseList.SetSize(msg.Width-h, msg.Height-v-4) // TODO: the 4 is the header view height
	}
	var cmd tea.Cmd
	m.testCaseList, cmd = m.testCaseList.Update(msg)
	return m, cmd
}
