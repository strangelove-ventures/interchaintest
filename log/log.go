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

// Format is the log output format
type Format int

const (
	Console Format = iota
	JSON
)

// Level is the log level
type Level int8

const (
	DebugLevel Level = iota
	InfoLevel
	ErrorLevel
)

var lvlMapping = map[Level]zerolog.Level{
	DebugLevel: zerolog.DebugLevel,
	InfoLevel:  zerolog.InfoLevel,
	ErrorLevel: zerolog.ErrorLevel,
}

// New returns a valid Logger.
// Level can be error, info, debug.
func New(w io.Writer, format Format, level Level) Logger {
	lg := log.Output(zerolog.ConsoleWriter{Out: w})
	if format == JSON {
		lg = zerolog.New(w).With().Timestamp().Logger()
	}

	lg = lg.Level(lvlMapping[level])

	return logger{
		logger: lg,
	}
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
