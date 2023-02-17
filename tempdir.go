package interchaintest

import (
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TempDirTestingT is a subset of testing.TB to implement TempDir.
type TempDirTestingT interface {
	Helper()

	Name() string

	Failed() bool
	Cleanup(func())

	Logf(format string, args ...any)
	Errorf(format string, args ...any)
}

// keepTempDirOnFailure determines whether a directory created by TempDir
// is retained or deleted following a test failure.
//
// The KeepTempDirOnFailure function is the public API to access this value.
// We export the function instead of the package-level variable
// for a consistent API in interchaintest with the KeepDockerVolumesOnFailure function,
// which references a variable in an internal package.
var keepTempDirOnFailure = os.Getenv("IBCTEST_SKIP_FAILURE_CLEANUP") != ""

// KeepTempDirOnFailure sets whether a directory created by TempDir
// is retained or deleted following a test failure.
//
// The value is false by default, but can be initialized to true by setting the
// environment variable IBCTEST_SKIP_FAILURE_CLEANUP to a non-empty value.
// Alternatively, importers of the interchaintest package may set the variable to true.
func KeepTempDirOnFailure(b bool) {
	keepTempDirOnFailure = b
}

// KeepingTempDirOnFailure reports the current value of KeepTempDirOnFailure.
// This function is only intended for tests.
func KeepingTempDirOnFailure() bool {
	return keepTempDirOnFailure
}

// TempDir resembles (*testing.T).TempDir, except that it conditionally
// keeps the temporary directory on disk, and it uses a new temporary directory
// on each invocation instead of adjacent directories with an incrementing numeric suffix.
//
// If the test passes, or if KeepTempDirOnFailure is set to false,
// the directory will be removed.
func TempDir(t TempDirTestingT) string {
	t.Helper()

	dir, err := os.MkdirTemp("", sanitizeTestName(t.Name()))
	if err != nil {
		// Realistically this should never fail.
		// Panicking allows a slimmer TempDirTestingT interface
		// (because we don't need to include Fatalf).
		panic(fmt.Errorf("TempDir: %w", err))
	}

	t.Cleanup(func() {
		if keepTempDirOnFailure && t.Failed() {
			t.Logf("Not removing temporary directory for test at: %s", dir)
			return
		}

		if err := os.RemoveAll(dir); err != nil {
			// Same error message as (*testing.T).TempDir.
			// If the directory can't be cleaned up,
			// that usually indicates something is subtly wrong.
			// Most often it seems to be something still writing to the directory
			// even though the test has ostensibly finished.
			t.Errorf("TempDir RemoveAll cleanup: %v", err)
		}
	})

	return dir
}

func sanitizeTestName(name string) string {
	// Copied from the standard library's (*testing.T).Cleanup:

	// Drop unusual characters (such as path separators or
	// characters interacting with globs) from the directory name to
	// avoid surprising os.MkdirTemp behavior.
	mapper := func(r rune) rune {
		if r < utf8.RuneSelf {
			const allowed = "!#$%&()+,-.=@^_{}~ "
			if '0' <= r && r <= '9' ||
				'a' <= r && r <= 'z' ||
				'A' <= r && r <= 'Z' {
				return r
			}
			if strings.ContainsRune(allowed, r) {
				return r
			}
		} else if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}
	return strings.Map(mapper, name)
}
