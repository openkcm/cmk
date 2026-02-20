package eventprocessor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
)

func TestParseOrbitalError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected eventprocessor.Error
	}{
		{
			name:  "Should get error code and message",
			input: "TEST_CODE:TestMessage",
			expected: eventprocessor.Error{
				Code:    "TEST_CODE",
				Message: "TestMessage",
			},
		},
		{
			name:  "Should get message and default error code on non-existing code",
			input: "TestMessage",
			expected: eventprocessor.Error{
				Code:    constants.DefaultErrorCode,
				Message: "TestMessage",
			},
		},
		{
			name:  "Should have default code on non screaming snake case code",
			input: "test_code:TestMessage",
			expected: eventprocessor.Error{
				Code:    constants.DefaultErrorCode,
				Message: "test_code:TestMessage",
			},
		},
		{
			name:  "Should have default code on code with invalid characters",
			input: "TEST-CODE:TestMessage",
			expected: eventprocessor.Error{
				Code:    constants.DefaultErrorCode,
				Message: "TEST-CODE:TestMessage",
			},
		},
		{
			name:  "Should handle code and message containing the separator",
			input: "TEST_CODE:Test message : with the separator",
			expected: eventprocessor.Error{
				Code:    "TEST_CODE",
				Message: "Test message : with the separator",
			},
		},
		{
			name:  "Should handle no code and message containing the separator",
			input: "Test message : with the separator",
			expected: eventprocessor.Error{
				Code:    constants.DefaultErrorCode,
				Message: "Test message : with the separator",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := eventprocessor.ParseOrbitalError(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}
