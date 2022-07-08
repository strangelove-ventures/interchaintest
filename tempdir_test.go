package ibctest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/stretchr/testify/require"
)

type mockTempT struct {
	name string

	failed bool

	helperCalled bool

	cleanups []func()

	logs   []string
	errors []string
}

func (t *mockTempT) Helper() {
	t.helperCalled = true
}

func (t *mockTempT) Name() string {
	return t.name
}

func (t *mockTempT) Failed() bool {
	return t.failed
}

func (t *mockTempT) Cleanup(f func()) {
	t.cleanups = append(t.cleanups, f)
}

func (t *mockTempT) RunCleanups() {
	// Cleanups are run in reverse order of insertion.
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
	t.cleanups = nil
}

func (t *mockTempT) Logf(format string, args ...any) {
	t.logs = append(t.logs, fmt.Sprintf(format, args...))
}

func (t *mockTempT) Errorf(format string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func TestTempDir_Cleanup(t *testing.T) {
	origKeep := ibctest.KeepTempDirOnFailure
	defer func() {
		ibctest.KeepTempDirOnFailure = origKeep
	}()

	t.Run("keep=true", func(t *testing.T) {
		ibctest.KeepTempDirOnFailure = true

		t.Run("test passed", func(t *testing.T) {
			mt := &mockTempT{name: "t"}

			dir := ibctest.TempDir(mt)
			require.DirExists(t, dir)

			mt.RunCleanups()

			require.NoDirExists(t, dir)
			require.Empty(t, mt.logs)
		})

		t.Run("test failed", func(t *testing.T) {
			mt := &mockTempT{name: "t"}

			dir := ibctest.TempDir(mt)
			require.DirExists(t, dir)
			defer func() { _ = os.RemoveAll(dir) }()

			mt.failed = true

			mt.RunCleanups()

			// Directory still exists after cleanups.
			require.DirExists(t, dir)

			// And the last log message mentions the directory.
			require.NotEmpty(t, mt.logs)
			require.Contains(t, mt.logs[len(mt.logs)-1], dir)
		})
	})

	t.Run("keep=false", func(t *testing.T) {
		ibctest.KeepTempDirOnFailure = false

		for name, failed := range map[string]bool{
			"test passed": false,
			"test failed": true,
		} {
			failed := failed
			t.Run(name, func(t *testing.T) {
				mt := &mockTempT{name: "t"}

				dir := ibctest.TempDir(mt)
				require.DirExists(t, dir)

				mt.failed = failed

				mt.RunCleanups()

				require.NoDirExists(t, dir)
				require.Empty(t, mt.logs)
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
			dir := ibctest.TempDir(t)

			require.Truef(
				t,
				strings.HasPrefix(dir, wantDir),
				"directory %s should have started with %s", dir, wantDir,
			)
		})
	}
}
