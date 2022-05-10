package log

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// mimics stdlib Lshortfile
	zerolog.CallerMarshalFunc = func(file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
}

// New returns a valid Logger.
// Format must be one of: console or json.
// Level must be one of: error, info, or debug.
func New(w io.Writer, format string, level string) Logger {
	lg := log.Output(zerolog.ConsoleWriter{Out: w})
	if format == "json" {
		lg = zerolog.New(w).With().Timestamp().Logger()
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	lg = lg.Level(lvl)

	return logger{
		logger: lg,
	}
}

// Nop returns a no-op logger
func Nop() Logger {
	return New(io.Discard, "console", "error")
}

type logger struct {
	logger zerolog.Logger
}

// WithField returns a new logger with attached key-value pair as a field
func (l logger) WithField(key string, val interface{}) Logger {
	return logger{
		logger: l.logger.With().Interface(key, val).Logger(),
	}
}

func (l logger) Debug(args ...interface{}) {
	l.send(l.logger.Debug(), args...)
}

func (l logger) Debugf(format string, args ...interface{}) {
	l.sendf(l.logger.Debug(), format, args...)
}

func (l logger) Info(args ...interface{}) {
	l.send(l.logger.Info(), args...)
}

func (l logger) Infof(format string, args ...interface{}) {
	l.sendf(l.logger.Info(), format, args...)
}

func (l logger) Error(args ...interface{}) {
	l.send(l.logger.Error(), args...)
}

func (l logger) Errorf(format string, args ...interface{}) {
	l.sendf(l.logger.Error(), format, args...)
}

func (l logger) Level() string {
	return l.logger.GetLevel().String()
}

func (l logger) send(event *zerolog.Event, args ...interface{}) {
	event = event.Caller(2)
	if len(args) == 0 {
		event.Send()
		return
	}
	event.Msg(strings.TrimSpace(fmt.Sprintln(args...)))
}

func (l logger) sendf(event *zerolog.Event, format string, args ...interface{}) {
	event = event.Caller(2)
	if len(args) == 0 {
		event.Send()
		return
	}
	event.Msgf(format, args...)
}
