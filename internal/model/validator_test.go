package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestValidateAll(t *testing.T) {
	tests := map[string]struct {
		validators []model.Validator
		expectErr  bool
	}{
		"No error expected with empty validators": {
			validators: []model.Validator{},
			expectErr:  false,
		},
		"Error expected with validators that do return an error": {
			validators: []model.Validator{&model.Tenant{}},
			expectErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := model.ValidateAll(test.validators...)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
