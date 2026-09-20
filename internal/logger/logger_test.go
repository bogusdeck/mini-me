package logger_test

import (
	"testing"

	"memex/internal/logger"
)

func TestLoggerInit(t *testing.T) {
	logger.Init(true)
	if logger.L() == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.Debug("test debug message", "key", "value")
	logger.Info("test info message")
}
