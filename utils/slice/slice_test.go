package slice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/slice"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    any
		item     any
		expected bool
	}{
		{
			name:     "String slice with item",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "String slice without item",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "grape",
			expected: false,
		},
		{
			name:     "Empty string slice",
			slice:    []string{},
			item:     "apple",
			expected: false,
		},
		{
			name:     "Int slice with item",
			slice:    []int{1, 2, 3, 4, 5},
			item:     3,
			expected: true,
		},
		{
			name:     "Int slice without item",
			slice:    []int{1, 2, 3, 4, 5},
			item:     6,
			expected: false,
		},
		{
			name:     "Single element string slice with item",
			slice:    []string{"apple"},
			item:     "apple",
			expected: true,
		},
		{
			name:     "Single element string slice without item",
			slice:    []string{"apple"},
			item:     "banana",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch s := tt.slice.(type) {
			case []string:
				item, ok := tt.item.(string)
				assert.True(t, ok)
				assert.Equal(t, tt.expected, slice.Contains(s, item))
			case []int:
				item, ok := tt.item.(int)
				assert.True(t, ok)
				assert.Equal(t, tt.expected, slice.Contains(s, item))
			}
		})
	}
}
