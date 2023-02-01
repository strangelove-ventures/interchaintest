package interchaintest

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultBlockDatabaseFilepath(t *testing.T) {
	got := DefaultBlockDatabaseFilepath()
	parts := strings.Split(got, string(os.PathSeparator))

	require.NotEmpty(t, parts)
	require.Equal(t, []string{".interchaintest", "databases", "block.db"}, parts[len(parts)-3:])
}
