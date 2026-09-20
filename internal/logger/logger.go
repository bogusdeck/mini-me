package logger

import (
	"log/slog"
	"os"
	"sync/atomic"
)

var globalLogger atomic.Pointer[slog.Logger]

func init() {
	// Initialize default logger to TextHandler writing to stderr
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	l := slog.New(handler)
	globalLogger.Store(l)
}

// Init initializes the global logger with debug mode optional.
func Init(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	l := slog.New(handler)
	globalLogger.Store(l)
}

// L returns the active global slog.Logger.
func L() *slog.Logger {
	return globalLogger.Load()
}

// Debug logs at LevelDebug.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at LevelInfo.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at LevelWarn.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at LevelError.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}
