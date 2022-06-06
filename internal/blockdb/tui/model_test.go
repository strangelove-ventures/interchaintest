package tui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type mockQueryService struct {
}

func TestModel_Init(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		(&Model{
			//QueryService:    &mockQueryService{},
			DBFilePath:   "/some/path.db",
			SchemaGitSha: "sha",
		}).Init()
	})

	require.Panics(t, func() {
		(&Model{}).Init()
	})
}

func TestModel_View(t *testing.T) {
	t.Parallel()

	t.Run("schema version", func(t *testing.T) {
		m := &Model{DBFilePath: "path.db", SchemaGitSha: "git-sha-123"}
		require.Regexp(t, `Database:.*path.db`, m.View())
		require.Regexp(t, `Schema Version:.*git-sha-123`, m.View())
	})
}
