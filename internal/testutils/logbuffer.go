package testutils

import (
	"bytes"
	"log/slog"
)

var logBuffer bytes.Buffer

// SetupLoggerWithBuffer returns a logger that writes to a buffer
func SetupLoggerWithBuffer() *slog.Logger {
	handler := slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{})
	logger := slog.New(handler)

	return logger
}
