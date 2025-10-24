package must_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/must"
)

var (
	errForced = errors.New("test error")
	errNext   = errors.New("another error")
)

// TestNotReturnError tests the NotReturnError function with various inputs and errors.
func TestNotReturnError(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		err       error
		wantPanic bool
	}{
		{
			name:      "noErrorReturnValue",
			value:     42,
			err:       nil,
			wantPanic: false,
		},
		{
			name:      "errorShouldPanic",
			value:     nil,
			err:       errForced,
			wantPanic: true,
		},
		{
			name:      "noErrorReturnString",
			value:     "hello",
			err:       nil,
			wantPanic: false,
		},
		{
			name:      "errorShouldPanicWithString",
			value:     "world",
			err:       errNext,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					must.NotReturnError(tt.value, tt.err)
				}, "Expected panic but function did not panic")
			} else {
				assert.NotPanics(t, func() {
					got := must.NotReturnError(tt.value, tt.err)
					assert.Equal(t, tt.value, got, "Expected value does not match")
				}, "Function panicked but it should not have")
			}
		})
	}
}
