package dockerutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainerJob_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	pool, networkID := DockerSetup(t)

	t.Run("happy path", func(t *testing.T) {
		job := NewContainerJob(pool, networkID, "busybox", "latest")
		stdout, stderr, err := job.Run(ctx, "test@happy|path", []string{"echo", "-n", "hello"}, JobOptions{})

		require.NoError(t, err)
		require.Equal(t, "hello", string(stdout))
		require.Empty(t, string(stderr))

	})

	t.Run("binds", func(t *testing.T) {
		job := NewContainerJob(pool, networkID, "busybox", "latest")

		const scriptBody = `#!/bin/sh
echo -n hi from stderr >> /dev/stderr
`
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "test.sh"), []byte(scriptBody), 0777)
		require.NoError(t, err)

		t.Log("temp", tmpDir)

		opts := JobOptions{
			Binds: []string{tmpDir + ":/test"},
		}

		stdout, stderr, err := job.Run(ctx, "binds", []string{"/test/test.sh"}, opts)
		require.NoError(t, err)
		require.Empty(t, string(stdout))
		require.Equal(t, "hi from stderr", string(stderr))
	})

	t.Run("context cancelled", func(t *testing.T) {
		job := NewContainerJob(pool, networkID, "busybox", "latest")
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := job.Run(cctx, "test context", []string{"sleep", "100"}, JobOptions{})

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("errors", func(t *testing.T) {

	})

	t.Run("command does not exist", func(t *testing.T) {
		// Using gaia to simulate a real use case.
		//job := NewContainerJob(pool, networkID, "", "latest")

	})
	t.Run("missing required fields", func(t *testing.T) {
		t.Fail()
	})
}
