package model_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

// errUnrelated is a non-validation sentinel used to assert that unrelated errors
// are not misclassified as model validation failures.
var errUnrelated = errors.New("some other error")

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

// TestValidationErrorsWrapErrValidation guards the invariant that every model-level
// validation error wraps model.ErrValidation, so callers (e.g. the tenant operator)
// can classify all of them as terminal via errors.Is(err, model.ErrValidation).
func TestValidationErrorsWrapErrValidation(t *testing.T) {
	validationErrs := map[string]error{
		"ErrInvalidIAMIdentifier":        model.ErrInvalidIAMIdentifier,
		"ErrInvalidName":                 model.ErrInvalidName,
		"ErrInvalidTenantRole":           model.ErrInvalidTenantRole,
		"ErrInvalidTenantStatus":         model.ErrInvalidTenantStatus,
		"ErrInvalidWorkflowState":        model.ErrInvalidWorkflowState,
		"ErrInvalidWorkflowArtifactType": model.ErrInvalidWorkflowArtifactType,
		"ErrInvalidWorkflowActionType":   model.ErrInvalidWorkflowActionType,
	}

	for name, err := range validationErrs {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, err, model.ErrValidation,
				"%s must wrap model.ErrValidation", name)
		})
	}

	// Sharing a parent sentinel must not collapse the siblings into one another:
	// each specific error must remain distinguishable so existing errors.Is checks
	// (e.g. the apierrors mapper distinguishing ErrInvalidName from ErrInvalidIAMIdentifier)
	// keep working.
	assert.NotErrorIs(t, model.ErrInvalidName, model.ErrInvalidIAMIdentifier)
	assert.NotErrorIs(t, model.ErrInvalidTenantRole, model.ErrInvalidTenantStatus)
	assert.NotErrorIs(t, model.ErrInvalidWorkflowState, model.ErrInvalidWorkflowActionType)

	// A non-validation error must not be classified as a validation failure.
	assert.NotErrorIs(t, errUnrelated, model.ErrValidation)
}
