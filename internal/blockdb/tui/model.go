package tui

import (
	"errors"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// QueryService queries a database and returns results.
type QueryService interface {
}

// Model is a tea.Model.
// The struct must be initialized with all exported fields set to non-empty values.
type Model struct {
	// Path to the sqlite database
	DBFilePath   string
	SchemaGitSha string
	TestCases    []blockdb.TestCaseResult

	testCaseList list.Model
}

// Init implements tea.Model.
// Init panics if any exported field is not set.
func (m *Model) Init() tea.Cmd {
	if m.DBFilePath == "" {
		panic(errors.New("missing DBFilePath"))
	}
	if m.SchemaGitSha == "" {
		panic(errors.New("missing SchemaGitSha"))
	}
	m.testCaseList = newListModel("Select Test Case", testCasesToItems(m.TestCases))
	return nil
}

// View implements tea.Model.
func (m *Model) View() string {
	return docStyle.Render(
		lipgloss.JoinVertical(0,
			schemaVersionView(m.DBFilePath, m.SchemaGitSha), m.testCaseList.View(),
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
