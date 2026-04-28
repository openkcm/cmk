package errors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	err "github.com/openkcm/cmk/internal/apiregistry/errors"
)

var errStandard = errors.New("standard error")

func TestNewValidationError(t *testing.T) {
	field := "username"
	message := "must not be empty"

	result := err.NewValidationError(field, message)

	assert.Error(t, result, "NewValidationError should return an error")

	expectedErr := fmt.Sprintf("validation error on field '%s': %s", field, message)
	assert.Equal(t, expectedErr, result.Error())

	// Verify it's recognized as a ValidationError
	assert.True(t, err.IsValidationError(result), "NewValidationError result should be recognized by IsValidationError")
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name          string
		field         string
		message       string
		expectedError string
	}{
		{
			name:          "standard error message",
			field:         "email",
			message:       "invalid format",
			expectedError: "validation error on field 'email': invalid format",
		},
		{
			name:          "empty field",
			field:         "",
			message:       "some message",
			expectedError: "validation error on field '': some message",
		},
		{
			name:          "empty message",
			field:         "password",
			message:       "",
			expectedError: "validation error on field 'password': ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := err.NewValidationError(tt.field, tt.message)

			assert.Equal(t, tt.expectedError, result.Error())
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ValidationError",
			err:      err.NewValidationError("field", "message"),
			expected: true,
		},
		{
			name:     "ValidationError pointer",
			err:      err.NewValidationError("test", "test message"),
			expected: true,
		},
		{
			name:     "standard error",
			err:      errStandard,
			expected: false,
		},
		{
			name:     "wrapped ValidationError with fmt.Errorf",
			err:      fmt.Errorf("wrapped: %w", err.NewValidationError("field", "message")),
			expected: true,
		},
		{
			name:     "wrapped standard error",
			err:      fmt.Errorf("wrapped: %w", errStandard),
			expected: false,
		},
		{
			name:     "deeply wrapped ValidationError",
			err:      fmt.Errorf("level 2: %w", fmt.Errorf("level 1: %w", err.NewValidationError("field", "message"))),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := err.IsValidationError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
