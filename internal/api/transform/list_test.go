package transform_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/transform"
)

var (
	ErrForced = errors.New("test error")
)

// transformIntToString is a test function that transforms an integer to a string.
// It returns an error if the input is 3.
func transformIntToString(i int) (*string, error) {
	if i == 3 {
		return nil, ErrForced
	}

	s := string(rune(i + '0')) //nolint:gosec // i is bounded 0-9, no overflow possible

	return &s, nil
}

// TestToList tests the ToList function.
func TestToList(t *testing.T) {
	tests := []struct {
		name     string
		items    []*int
		expected []string
		err      error
	}{
		{
			name:     "TransformsItemsSuccessfully",
			items:    []*int{new(1), new(2)},
			expected: []string{"1", "2"},
			err:      nil,
		},
		{
			name:     "HandlesTransformationError",
			items:    []*int{new(1), new(2), new(3), new(4)},
			expected: nil,
			err:      ErrForced,
		},
		{
			name:     "HandlesEmptyInput",
			items:    []*int{},
			expected: []string{},
			err:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transform.ToList(tt.items, transformIntToString)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
