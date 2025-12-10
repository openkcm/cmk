package apierrors_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/utils/ptr"
)

var (
	ErrForced      = errors.New("forced error")
	ErrKeyConfig   = errors.New("key config error")
	ErrKeyVersion  = errors.New("key version error")
	ErrNonMatching = errors.New("non matching error")

	ErrTest       = errors.New("test error")
	ErrAnother    = errors.New("another error")
	ErrYetAnother = errors.New("yet another error")

	ErrSome = errors.New("some error")
)

func TestMapErrorToResponse(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected cmkapi.ErrorMessage
	}{
		{
			name:     "UnmappedError",
			err:      ErrForced,
			expected: apierrors.InternalServerErrorMessage(),
		},
		{
			name: "ValidError",
			err:  errs.Wrap(manager.ErrKeyConfigurationNotFound, gorm.ErrRecordNotFound),
			expected: cmkapi.ErrorMessage{
				Error: cmkapi.DetailedError{
					Code:    "KEY_CONFIGURATION_NOT_FOUND",
					Message: "fail to get system by KeyConfigurationID",
					Status:  http.StatusNotFound,
				},
			},
		},
		{
			name: "ValidWrappedError",
			err:  fmt.Errorf("%w %w", manager.ErrGettingKeyByID, repo.ErrNotFound),
			expected: cmkapi.ErrorMessage{
				Error: cmkapi.DetailedError{
					Code:    "RESOURCE_NOT_FOUND",
					Message: "The requested resource was not found",
					Status:  http.StatusNotFound,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apierrors.TransformToAPIError(t.Context(), tt.err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.Error.Code, result.Error.Code)
			assert.Equal(t, tt.expected.Error.Message, result.Error.Message)
			assert.Equal(t, tt.expected.Error.Status, result.Error.Status)
		})
	}
}

func TestMatchingErrors(t *testing.T) {
	candidates := []error{ErrTest, ErrAnother}
	tests := []struct {
		name       string
		err        error
		candidates []error
		expected   int
	}{
		{
			name:       "NoFullMatch",
			err:        ErrTest,
			candidates: candidates,
			expected:   1,
		},
		{
			name:       "Matches",
			err:        fmt.Errorf("%w %w", ErrTest, ErrAnother),
			candidates: candidates,
			expected:   2,
		},
		{
			name:       "MatchesJoined",
			err:        errors.Join(ErrTest, ErrAnother),
			candidates: candidates,
			expected:   2,
		},
		{
			name:       "NoMatch",
			err:        ErrYetAnother,
			candidates: candidates,
			expected:   0,
		},
		{
			name:       "EmptyCandidates",
			err:        ErrTest,
			candidates: []error{},
			expected:   0,
		},
		{
			name:       "NilError",
			err:        nil,
			candidates: candidates,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apierrors.CountMatchingErrors(tt.err, tt.candidates)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapErrorToResponseNoMapping(t *testing.T) {
	requestID := uuid.NewString()
	tests := []struct {
		name        string
		operationID string
		err         error
		expected    cmkapi.ErrorMessage
	}{
		{
			name:        "UnknownOperation",
			operationID: "unknownOperation",
			err:         ErrSome,
			expected:    apierrors.InternalServerErrorMessage(),
		},
		{
			name:        "WithMapping",
			operationID: "GetKeys",
			err:         apierrors.ErrTransformKeyToAPI,
			expected: cmkapi.ErrorMessage{
				Error: cmkapi.DetailedError{
					Code:      "TRANSFORM_KEY",
					Message:   "Failed to transform key",
					Status:    http.StatusInternalServerError,
					RequestID: ptr.PointTo(requestID),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apierrors.TransformToAPIError(t.Context(), tt.err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.Error.Code, result.Error.Code)
			assert.Equal(t, tt.expected.Error.Message, result.Error.Message)
			assert.Equal(t, tt.expected.Error.Status, result.Error.Status)
		})
	}
}

func TestMapErrorToResponseWithContextGetter(t *testing.T) {
	expectedContext := map[string]any{"foo": "bar"}

	apiErrorMapper := apierrors.APIErrorMapper{
		APIErrors: []apierrors.APIErrors{
			{
				Errors:       []error{ErrTest},
				ExposedError: cmkapi.DetailedError{Code: "CODE", Message: "Some error", Status: 400},
				ContextGetter: func(_ error) map[string]any {
					return expectedContext
				},
			},
		},
	}

	result := apiErrorMapper.Transform(t.Context(), ErrTest)
	assert.NotNil(t, result)
	assert.Equal(t, "CODE", result.Error.Code)
	assert.Equal(t, expectedContext, *result.Error.Context)
}

func TestMatchedErrorLogicWithEqualPriority(t *testing.T) {
	apiErrorMapper := apierrors.APIErrorMapper{
		APIErrors: []apierrors.APIErrors{
			{
				Errors:       []error{apierrors.ErrCreateKey, repo.ErrCreateResource},
				ExposedError: cmkapi.DetailedError{Code: "ERROR_2", Message: "Matched error 2", Status: 500},
			},
			{
				Errors:       []error{apierrors.ErrCreateKey, apierrors.ErrCreateKeyVersion},
				ExposedError: cmkapi.DetailedError{Code: "ERROR_3", Message: "Matched error 3", Status: 400},
			},
			{
				Errors:       []error{apierrors.ErrCreateKeyVersion},
				ExposedError: cmkapi.DetailedError{Code: "ERROR_4", Message: "Matched error 4", Status: 400},
			},
			{
				Errors:       []error{apierrors.ErrCreateKey, repo.ErrCreateResource},
				ExposedError: cmkapi.DetailedError{Code: "ERROR_5", Message: "Matched error 5", Status: 400},
			},
		},
		PriorityErrors: []apierrors.APIErrors{
			{
				Errors:       []error{repo.ErrTenantNotFound},
				ExposedError: cmkapi.DetailedError{Code: "ERROR_1", Message: "Matched error 1", Status: 404},
			},
		},
	}

	tests := []struct {
		name         string
		err          error
		expectedCode string
	}{
		{
			name:         "Should match based on priority",
			err:          errs.Wrap(apierrors.ErrCreateKey, errs.Wrap(repo.ErrTenantNotFound, repo.ErrCreateResource)),
			expectedCode: "ERROR_1",
		},
		{
			name:         "Should match on most similar matching chain",
			err:          errs.Wrap(apierrors.ErrCreateKey, apierrors.ErrCreateKeyVersion),
			expectedCode: "ERROR_3",
		},
		{
			name:         "Should match on fewer amount of errors on multiple matching chains",
			err:          apierrors.ErrCreateKeyVersion,
			expectedCode: "ERROR_4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apiErrorMapper.Transform(t.Context(), tt.err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCode, result.Error.Code)
		})
	}
}
