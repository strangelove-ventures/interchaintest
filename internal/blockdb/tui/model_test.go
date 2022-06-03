package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockQueryService struct {
}

func TestModel_Init(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		(&Model{
			QueryService:    &mockQueryService{},
			SchemaCreatedAt: time.Now(),
			SchemaGitSha:    "sha",
		}).Init()
	})

	require.Panics(t, func() {
		(&Model{}).Init()
	})
}

func TestModel_View(t *testing.T) {
	t.Parallel()

	t.Run("schema version", func(t *testing.T) {
		now := time.Now()
		m := &Model{SchemaCreatedAt: now, SchemaGitSha: "git-sha-123"}
		require.Contains(t, m.View(), now.Format(time.RFC822))
		require.Contains(t, m.View(), "git-sha-123")
	})
}
