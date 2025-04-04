package cardano

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
)

// zapHandler implements slog.Handler using zap.Logger
type zapHandler struct {
	log *zap.Logger
}

// Enabled implements slog.Handler.
func (h *zapHandler) Enabled(_ context.Context, level slog.Level) bool {
	// This could be improved to check zap's level configuration
	return true
}

// Handle implements slog.Handler.
func (h *zapHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make([]zap.Field, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, zap.Any(a.Key, a.Value.Any()))
		return true
	})

	switch r.Level {
	case slog.LevelDebug:
		h.log.Debug(r.Message, fields...)
	case slog.LevelInfo:
		h.log.Info(r.Message, fields...)
	case slog.LevelWarn:
		h.log.Warn(r.Message, fields...)
	case slog.LevelError:
		h.log.Error(r.Message, fields...)
	default:
		h.log.Info(r.Message, fields...)
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (h *zapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Create fields from attrs
	fields := make([]zap.Field, 0, len(attrs))
	for _, attr := range attrs {
		fields = append(fields, zap.Any(attr.Key, attr.Value.Any()))
	}

	// Create a new logger with fields
	newLogger := h.log.With(fields...)
	return &zapHandler{log: newLogger}
}

// WithGroup implements slog.Handler.
func (h *zapHandler) WithGroup(name string) slog.Handler {
	// For simplicity, we can implement groups as prefixes in field names
	// A more sophisticated implementation might use zap's namespaces
	return h
}

// NewSlogWrapper creates a new slog.Logger that uses zap.Logger internally.
func NewSlogWrapper(log *zap.Logger) *slog.Logger {
	return slog.New(&zapHandler{log: log})
}
