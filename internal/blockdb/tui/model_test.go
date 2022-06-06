package tui

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
	"github.com/stretchr/testify/require"
)

type mockQueryService struct {
}

func (m mockQueryService) Chains(ctx context.Context, testCaseID int64) ([]blockdb.ChainResult, error) {
	//TODO implement me
	panic("implement me")
}

func TestNewModel(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		NewModel(
			context.Background(),
			&mockQueryService{},
			"/path/to/test.db",
			"abc123",
			nil)
	})

	require.Panics(t, func() {
		NewModel(nil, nil, "", "", nil)
	})
}

func TestModel_Init(t *testing.T) {
	t.Parallel()

	require.Nil(t, (&Model{}).Init())
}

func TestModel_Update(t *testing.T) {
	t.Parallel()

	t.Run("quit keys", func(t *testing.T) {
		t.Fatal("TODO")
	})
}

func TestModel_View(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("schema version", func(t *testing.T) {
		m := NewModel(ctx, &mockQueryService{}, "path.db", "git-sha-123", nil)
		view := m.View()
		require.Regexp(t, `Database:.*path.db`, view)
		require.Regexp(t, `Schema Version:.*git-sha-123`, view)
	})

	t.Run("initial test cases", func(t *testing.T) {
		tc := []blockdb.TestCaseResult{
			{Name: "test1", GitSha: "sha1"},
			{Name: "test2", GitSha: "sha2"},
		}
		m := NewModel(ctx, &mockQueryService{}, "path", "sha", tc)

		m.testCaseList.SetWidth(500)
		m.testCaseList.SetHeight(500)
		view := m.View()

		require.Contains(t, view, "test1 (git: sha1)")
		require.Contains(t, view, "test2 (git: sha2)")
	})
}
