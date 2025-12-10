package constants_test

import (
	"testing"

	"github.tools.sap/kms/cmk/internal/constants"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		logLevel constants.LogLevel
		expected string
	}{
		{"Debug level", constants.LogLevelDebug, "debug"},
		{"Info level", constants.LogLevelInfo, "info"},
		{"Warn level", constants.LogLevelWarn, "warn"},
		{"Error level", constants.LogLevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.logLevel.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
