package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type WriteSyncer = zapcore.WriteSyncer

// New returns a valid Logger.
// Format must be one of: console or json.
// Level must be one of: error, info, or debug.
func New(w WriteSyncer, format string, level string) Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	enc := zapcore.NewConsoleEncoder(config)
	if format == "json" {
		enc = zapcore.NewJSONEncoder(config)
	}

	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logger := zap.New(zapcore.NewCore(enc, w, lvl)).Sugar()
	return zapLogger{logger, lvl.Level()}
}

// Nop returns a no-op logger
func Nop() Logger {
	return zapLogger{zap.NewNop().Sugar(), zapcore.ErrorLevel}
}

type zapLogger struct {
	logger *zap.SugaredLogger
	level  zapcore.Level
}

func (z zapLogger) With(key string, val interface{}) Logger {
	return zapLogger{z.logger.With(zap.Any(key, val)), z.level}
}

func (z zapLogger) Debug(args ...interface{}) {
	z.logger.Debug(args...)
}

func (z zapLogger) Debugf(format string, args ...interface{}) {
	z.logger.Debugf(format, args...)
}

func (z zapLogger) Info(args ...interface{}) {
	z.logger.Info(args...)
}

func (z zapLogger) Infof(format string, args ...interface{}) {
	z.logger.Infof(format, args...)
}

func (z zapLogger) Error(args ...interface{}) {
	z.logger.Error(args...)
}

func (z zapLogger) Errorf(format string, args ...interface{}) {
	z.logger.Errorf(format, args...)
}

func (z zapLogger) Level() string {
	return z.level.String()
}

func (z zapLogger) Flush() error {
	return z.logger.Sync()
}
