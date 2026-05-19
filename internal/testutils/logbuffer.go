package testutils

import (
	"bytes"
	"log/slog"

	slogctx "github.com/veqryn/slog-context"
)

// NewLogBuffer returns a slog.Logger and a buffer that captures log output.
// Use slog.SetDefault(logger) to capture logs from code using the default logger.
func NewLogBuffer() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slogctx.NewHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{}), nil)
	return slog.New(handler), buf
}

var logBuffer bytes.Buffer

// SetupLoggerWithBuffer returns a logger that writes to a buffer
func SetupLoggerWithBuffer() *slog.Logger {
	handler := slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{})
	logger := slog.New(handler)

	return logger
}
