package tui

import (
	"testing"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
	"github.com/stretchr/testify/require"
)

type mockQueryService struct {
}

func validModel() *Model {
	return &Model{
		DBFilePath:   "/path/to/test.db",
		SchemaGitSha: "abc123",
	}
}

func TestModel_Init(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		validModel().Init()
	})

	require.Panics(t, func() {
		(&Model{}).Init()
	})
}

func TestModel_Update(t *testing.T) {
	t.Run("quit keys", func(t *testing.T) {

	})
}

func TestModel_View(t *testing.T) {
	t.Parallel()

	t.Run("schema version", func(t *testing.T) {
		m := &Model{DBFilePath: "path.db", SchemaGitSha: "git-sha-123"}
		m.Init()
		view := m.View()
		require.Regexp(t, `Database:.*path.db`, view)
		require.Regexp(t, `Schema Version:.*git-sha-123`, view)
	})

	t.Run("initial test cases", func(t *testing.T) {
		m := validModel()
		m.TestCases = []blockdb.TestCaseResult{
			{Name: "test1", GitSha: "sha1"},
			{Name: "test2", GitSha: "sha2"},
		}
		m.Init()
		m.testCaseList.SetWidth(500)
		m.testCaseList.SetHeight(500)
		view := m.View()

		require.Contains(t, view, "test1 (git: sha1)")
		require.Contains(t, view, "test2 (git: sha2)")
	})
}
