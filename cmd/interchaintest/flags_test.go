package interchaintest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainFlags_Logger(t *testing.T) {
	for _, tt := range []struct {
		LogFile string
	}{
		{""},
		{"stdout"},
		{"stderr"},
	} {
		flags := mainFlags{LogFile: tt.LogFile}
		logger, err := flags.Logger()

		require.NoError(t, err)
		require.NoError(t, logger.Close())
		require.NotEmpty(t, logger.FilePath)
	}
}
