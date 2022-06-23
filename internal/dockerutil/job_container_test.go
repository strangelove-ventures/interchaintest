package dockerutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewJobContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	pool, networkID := DockerSetup(t)

	for _, tt := range []struct {
		Pool       *dockertest.Pool
		NetworkID  string
		Repository string
		Tag        string
	}{
		{nil, networkID, "repo", "tag"},
		{pool, "", "repo", "tag"},
		{pool, networkID, "", "tag"},
		{pool, networkID, "repo", ""},
	} {
		require.Panics(t, func() {
			NewJobContainer(zap.NewNop(), tt.Pool, tt.NetworkID, tt.Repository, tt.Tag)
		})
	}
}

func TestContainerJob_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	const (
		testImage = "busybox"
		testTag   = "latest"
	)

	ctx := context.Background()
	pool, networkID := DockerSetup(t)

	// Ensure we have busybox.
	job := NewJobContainer(zap.NewNop(), pool, networkID, testImage, testTag)
	require.NoError(t, job.Pull(ctx))

	t.Run("happy path", func(t *testing.T) {
		stdout, stderr, err := job.Run(ctx, "test@happy|path", []string{"echo", "-n", "hello"}, JobOptions{})

		require.NoError(t, err)
		require.Equal(t, "hello", string(stdout))
		require.Empty(t, string(stderr))
	})

	t.Run("binds", func(t *testing.T) {
		const scriptBody = `#!/bin/sh
echo -n hi from stderr >> /dev/stderr
`
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "test.sh"), []byte(scriptBody), 0777)
		require.NoError(t, err)

		opts := JobOptions{
			Binds: []string{tmpDir + ":/test"},
		}

		stdout, stderr, err := job.Run(ctx, "binds", []string{"/test/test.sh"}, opts)
		require.NoError(t, err)
		require.Empty(t, string(stdout))
		require.Equal(t, "hi from stderr", string(stderr))
	})

	t.Run("context cancelled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := job.Run(cctx, "test context", []string{"sleep", "100"}, JobOptions{})

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("errors", func(t *testing.T) {
		_, _, err := job.Run(ctx, "errors", []string{"program-does-not-exist"}, JobOptions{})

		require.Error(t, err)
	})

	t.Run("command does not exist", func(t *testing.T) {
		// Using gaia to simulate real scenario.
		gaiaJob := NewJobContainer(
			zap.NewNop(),
			pool,
			networkID,
			"ghcr.io/strangelove-ventures/heighliner/gaia",
			"v7.0.2",
		)

		require.NoError(t, gaiaJob.Pull(ctx))

		_, _, err := gaiaJob.Run(ctx, "gaia", []string{"gaiad", "this-subcommand-should-never-exist"}, JobOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "exit code 1")
		require.Contains(t, err.Error(), "unknown command")
	})

	t.Run("missing required args", func(t *testing.T) {
		require.PanicsWithError(t, "cmd cannot be empty", func() {
			_, _, _ = job.Run(ctx, "errors", nil, JobOptions{})
		})
	})
}
