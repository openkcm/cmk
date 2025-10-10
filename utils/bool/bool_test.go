package boolutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	boolutil "github.com/openkcm/cmk/utils/bool"
)

func TestAnyTrue(t *testing.T) {
	tests := []struct {
		name     string
		input    []bool
		expected bool
	}{
		{"Returns true if any value is true", []bool{false, true, false}, true},
		{"Returns false if all values are false", []bool{false, false, false}, false},
		{"Returns false for empty input", []bool{}, false},
		{"Handles single true value", []bool{true}, true},
		{"Handles single false value", []bool{false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolutil.AnyTrue(tt.input...)
			require.Equal(t, tt.expected, result)
		})
	}
}
