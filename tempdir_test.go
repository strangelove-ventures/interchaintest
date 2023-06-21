package interchaintest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v5"
	"github.com/strangelove-ventures/interchaintest/v5/internal/mocktesting"
	"github.com/stretchr/testify/require"
)

func TestTempDir_Cleanup(t *testing.T) {
	origKeep := interchaintest.KeepingTempDirOnFailure()
	defer func() {
		interchaintest.KeepTempDirOnFailure(origKeep)
	}()

	t.Run("keep=true", func(t *testing.T) {
		interchaintest.KeepTempDirOnFailure(true)

		t.Run("test passed", func(t *testing.T) {
			mt := mocktesting.NewT("t")

			dir := interchaintest.TempDir(mt)
			require.DirExists(t, dir)

			mt.RunCleanups()

			require.NoDirExists(t, dir)
			require.Empty(t, mt.Logs)
		})

		t.Run("test failed", func(t *testing.T) {
			mt := mocktesting.NewT("t")

			dir := interchaintest.TempDir(mt)
			require.DirExists(t, dir)
			defer func() { _ = os.RemoveAll(dir) }()

			mt.Fail()

			mt.RunCleanups()

			// Directory still exists after cleanups.
			require.DirExists(t, dir)

			// And the last log message mentions the directory.
			require.NotEmpty(t, mt.Logs)
			require.Contains(t, mt.Logs[len(mt.Logs)-1], dir)
		})
	})

	t.Run("keep=false", func(t *testing.T) {
		interchaintest.KeepTempDirOnFailure(false)

		for name, failed := range map[string]bool{
			"test passed": false,
			"test failed": true,
		} {
			failed := failed
			t.Run(name, func(t *testing.T) {
				mt := mocktesting.NewT("t")

				dir := interchaintest.TempDir(mt)
				require.DirExists(t, dir)

				if failed {
					mt.Fail()
				}

				mt.RunCleanups()

				require.NoDirExists(t, dir)
				require.Empty(t, mt.Logs)
			})
		}
	})
}

func TestTempDir_Naming(t *testing.T) {
	const testNamePrefix = "TestTempDir_Naming"
	tmpRoot := os.TempDir()

	for name, expDir := range map[string]string{
		"A":        "A",
		"Foo_Bar":  "Foo_Bar",
		"1/2 full": "12_full",
		"/..":      "..", // Gets prefix appended, so will not traverse upwards.
		"\x00\xFF": "",
	} {
		wantDir := filepath.Join(tmpRoot, testNamePrefix+expDir)
		t.Run(name, func(t *testing.T) {
			dir := interchaintest.TempDir(t)

			require.Truef(
				t,
				strings.HasPrefix(dir, wantDir),
				"directory %s should have started with %s", dir, wantDir,
			)
		})
	}
}
