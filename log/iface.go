package log

var _ Logger = logger{}

// Logger is a convenience interface
type Logger interface {
	WithField(key string, val interface{}) Logger

	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	// Level returns current log level as a lowercased string
	Level() string
}
