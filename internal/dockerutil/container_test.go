package dockerutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewFactory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	pool := DockerSetup(t)

	for _, tt := range []struct {
		Name       string
		Repository string
	}{
		{"", "repo"},
		{"test", ""},
	} {
		require.Panics(t, func() {
			NewFactory(zap.NewNop(), pool, tt.Name, tt.Repository, "")
		})
	}
}

func TestFactory_RunJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	const (
		testImage = "busybox"
		testTag   = "latest"
	)

	ctx := context.Background()
	pool := DockerSetup(t)

	// Ensure we have busybox.
	factory := NewFactory(zap.NewNop(), pool, fmt.Sprintf("%s@?#!", t.Name()), testImage, testTag)

	t.Run("happy path", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			result, err := factory.RunJob(ctx, []string{"echo", "-n", "hello"}, RunOptions{})

			require.NoError(t, err)
			require.Equal(t, "hello", string(result.Stdout))
			require.Empty(t, string(result.Stderr))
		}
	})

	t.Run("binds", func(t *testing.T) {
		const scriptBody = `#!/bin/sh
	echo -n hi from stderr >> /dev/stderr
	`
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "test.sh"), []byte(scriptBody), 0777)
		require.NoError(t, err)

		opts := RunOptions{
			Binds: []string{tmpDir + ":/test"},
		}

		res, err := factory.RunJob(ctx, []string{"/test/test.sh"}, opts)
		require.NoError(t, err)
		require.Empty(t, string(res.Stdout))
		require.Equal(t, "hi from stderr", string(res.Stderr))
	})

	t.Run("context cancelled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := factory.RunJob(cctx, []string{"sleep", "100"}, RunOptions{})

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("errors", func(t *testing.T) {
		for _, tt := range []struct {
			Args    []string
			WantErr string
		}{
			{[]string{"program-does-not-exist"}, "executable file not found"},
			{[]string{"sleep", "not-valid-arg"}, "sleep: invalid"},
		} {
			_, err := factory.RunJob(ctx, tt.Args, RunOptions{})

			require.Error(t, err, tt)
			require.Contains(t, err.Error(), tt.WantErr, tt)
		}
	})

	t.Run("missing required args", func(t *testing.T) {
		require.PanicsWithError(t, "cmd cannot be empty", func() {
			_, _ = factory.RunJob(ctx, nil, RunOptions{})
		})
	})
}
