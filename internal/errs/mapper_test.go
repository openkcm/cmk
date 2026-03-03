package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/zeebo/assert"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrTest       = errors.New("test error")
	ErrAnother    = errors.New("another error")
	ErrYetAnother = errors.New("yet another error")

	ErrSome = errors.New("some error")

	errA     = errors.New("errA")
	errB     = errors.New("errB")
	errC     = errors.New("errC")
	errD     = errors.New("errD")
	errEmpty = errors.New("")
)

type TestError struct {
	Code    string
	Message string
	Context *map[string]any
}

func (e *TestError) SetContext(context *map[string]any) {
	e.Context = context
}

func (e *TestError) DefaultError() *TestError {
	return &TestError{
		Code:    "Default_code",
		Message: "Default_message",
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
			result := errs.CountMatchingErrors(tt.err, tt.candidates)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapErrorToResponseWithContextGetter(t *testing.T) {
	expectedContext := map[string]any{"foo": "bar"}

	errorMapper := errs.NewMapper([]errs.ExposedErrors[*TestError]{
		{
			InternalErrorChain: []error{ErrTest},
			ExposedError:       &TestError{Code: "CODE", Message: "Some error"},
			ContextGetter: func(_ error) map[string]any {
				return expectedContext
			},
		},
	}, []errs.ExposedErrors[*TestError]{})

	result := errorMapper.Transform(t.Context(), ErrTest)
	assert.NotNil(t, result)
	assert.Equal(t, "CODE", result.Code)
	assert.Equal(t, expectedContext, *result.Context)
}

// CreateKey, CreateResource, CreateKeyVersion, TenantNotFound
func TestMatchedErrorLogicWithEqualPriority(t *testing.T) {
	errorMapper := errs.NewMapper([]errs.ExposedErrors[*TestError]{
		{
			InternalErrorChain: []error{errA, errB},
			ExposedError:       &TestError{Code: "", Message: "Matched error 2"},
		},
		{
			InternalErrorChain: []error{errA, errC},
			ExposedError:       &TestError{Code: "ERROR_3", Message: "Matched error 3"},
		},
		{
			InternalErrorChain: []error{errC},
			ExposedError:       &TestError{Code: "ERROR_4", Message: "Matched error 4"},
		},
		{
			InternalErrorChain: []error{errA, errB},
			ExposedError:       &TestError{Code: "ERROR_5", Message: "Matched error 5"},
		},
		{
			InternalErrorChain: []error{errA, errB},
			ExposedError:       &TestError{Code: "ERROR_5", Message: "Matched error 5"},
		},
	}, []errs.ExposedErrors[*TestError]{
		{
			InternalErrorChain: []error{errD},
			ExposedError:       &TestError{Code: "ERROR_1", Message: "Matched error 1"},
		},
	})

	tests := []struct {
		name         string
		err          error
		expectedCode string
	}{
		{
			name:         "Should match based on priority",
			err:          errs.Wrap(errA, errs.Wrap(errD, errB)),
			expectedCode: "ERROR_1",
		},
		{
			name:         "Should match on most similar matching chain",
			err:          errs.Wrap(errA, errC),
			expectedCode: "ERROR_3",
		},
		{
			name:         "Should match on fewer amount of errors on multiple matching chains",
			err:          errC,
			expectedCode: "ERROR_4",
		},
		{
			name:         "Should default on no match",
			err:          errEmpty,
			expectedCode: "Default_code",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorMapper.Transform(t.Context(), tt.err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCode, result.Code)
		})
	}
}
