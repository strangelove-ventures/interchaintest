package ibctest

import (
	"fmt"
	"io"
	"os"

	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/log"
)

// The value of the extra flags this test supports.
type mainFlags struct {
	LogFile    string
	LogFormat  string
	LogLevel   string
	MatrixFile string
}

func (f mainFlags) Logger() (lc LoggerCloser, _ error) {
	var w io.Writer
	switch f.LogFile {
	case "stderr", "":
		w = os.Stderr
		lc.FilePath = "stderr"
	case "stdout":
		w = os.Stdout
		lc.FilePath = "stdout"
	default:
		file, err := ibctest.CreateLogFile(f.LogFile)
		if err != nil {
			return lc, fmt.Errorf("create log file: %w", err)
		}
		w = file
		lc.Closer = file
		lc.FilePath = file.Name()
	}
	lc.Logger = log.New(w, f.LogFormat, f.LogLevel)
	return lc, nil
}

type LoggerCloser struct {
	log.Logger
	io.Closer
	FilePath string
}

func (lc LoggerCloser) Close() error {
	if lc.Closer == nil {
		return nil
	}
	return lc.Closer.Close()
}
