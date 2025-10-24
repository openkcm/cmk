package base62_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk-core/utils/base62"
)

func TestSchemaName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"test", "_0NXZ0B", false},
		{"", "", true},
		{"123_ABC", "_DJUQfaGZiB", false},
		{"~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~",
			"_ezPf5nf8zPf5nf8zPf5nf8zPf5nf8zPf5nf8zPf5nf8zPf5nf8zPf5nf", false},
		{"KMS_tenant_name1", "_xUWbh52X05WYuVGdfaqmWC", false},
		{"KMS_tenant_name2", "_yUWbh52X05WYuVGdfaqmWC", false},
		{"tenant128_id", "_kl2X4ITM05WYuVGd", false},
		{"tenant129_id", "_kl2X5ITM05WYuVGd", false},
	}
	for _, tt := range tests {
		encodedString, err := base62.EncodeSchemaNameBase62(tt.input)
		if tt.wantErr {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, tt.expected, encodedString)
	}
}

func TestSchemaNameBase62(t *testing.T) {
	tests := []struct {
		input         string
		wantErr       bool
		expectedError error
	}{
		{"test", false, nil},
		{"", true, base62.ErrEmptyTenantID},
		{"123_ABC", false, nil},
		{"~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~", false, nil},
		{"KMS_tenant_name1", false, nil},
		{"KMS_tenant_name2", false, nil},
	}

	for _, tt := range tests {
		encodedString, err := base62.EncodeSchemaNameBase62(tt.input)

		if tt.wantErr {
			if err == nil {
				t.Errorf("SchemaNameBase62(%q) expected error, got nil (result: %q)", tt.input, encodedString)
			}

			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected %v, got %v", tt.expectedError, err)
			}

			continue
		}

		if err != nil {
			t.Errorf("SchemaNameBase62(%q) unexpected error: %v", tt.input, err)
			continue
		}

		decodedString, err := base62.DecodeSchemaNameBase62(encodedString)
		require.NoError(t, err)
		assert.Equal(t, tt.input, decodedString)

		re := regexp.MustCompile(`^[_]{1}[0-9A-Za-z]{1,62}$`)

		if !re.MatchString(encodedString) {
			t.Errorf("Encoded string %q don't match regexp for base62", encodedString)
		}

		if len(encodedString) < 3 || len(encodedString) > 62 {
			t.Errorf("Encoded string %q has wrong length %d", encodedString, len(encodedString))
		}
	}
}
