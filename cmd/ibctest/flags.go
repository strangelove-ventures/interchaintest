package ibctest

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/strangelove-ventures/ibctest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// The value of the extra flags this test supports.
type mainFlags struct {
	LogFile           string
	LogFormat         string
	LogLevel          string
	MatrixFile        string
	ReportFile        string
	BlockDatabaseFile string
}

func (f mainFlags) Logger() (lc LoggerCloser, _ error) {
	var w zapcore.WriteSyncer
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
	lc.Logger = f.newZap(w)
	return lc, nil
}

func (f mainFlags) newZap(w zapcore.WriteSyncer) *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	enc := zapcore.NewConsoleEncoder(config)
	if f.LogFormat == "json" {
		enc = zapcore.NewJSONEncoder(config)
	}

	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(f.LogLevel)); err != nil {
		lvl = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return zap.New(zapcore.NewCore(enc, w, lvl))
}

type LoggerCloser struct {
	*zap.Logger
	io.Closer
	FilePath string
}

func (lc LoggerCloser) Close() error {
	// ignore error because of https://github.com/uber-go/zap/issues/880 with stderr/stdout
	_ = lc.Logger.Sync()
	if lc.Closer == nil {
		return nil
	}
	return lc.Closer.Close()
}
