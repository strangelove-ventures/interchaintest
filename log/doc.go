// Package log is the preferred structured logging package to use throughout this codebase.
//
// It abstracts specific log implementations in case we want to change the concrete logger later.
// E.g. zerolog, stdlib log, logrus, zap, etc.
// Currently, the underlying logger is zerolog.
//
// This package does not concern itself with log performance.
//
// Important: This package does not replace testing's log, i.e. t.Log and t.Logf. Test logs should still
// be used to inform the user of the test progress.
package log
