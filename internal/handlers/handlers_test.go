package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/write"
	"github.tools.sap/kms/cmk/internal/handlers"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

var (
	errFailedToParseRequest   = errors.New("failed to parse request")
	errInvalidFormat          = errors.New("param in invalid format")
	errFailedToEncodeResponse = errors.New("failed to encode response")
	errUnknown                = errors.New("unknown error")
)

// Helper function to decode response
func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) cmkapi.ErrorMessage {
	t.Helper()

	var errorResponse cmkapi.ErrorMessage

	err := json.NewDecoder(recorder.Body).Decode(&errorResponse)
	assert.NoError(t, err)

	return errorResponse
}

// Test RequestErrorHandlerFunc
func TestRequestErrorHandlerFunc(t *testing.T) {
	handler := handlers.RequestErrorHandlerFunc()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/test-url", nil)

	handler(recorder, request, errFailedToParseRequest)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	errorResponse := decodeResponse(t, recorder)
	assert.Equal(t, "Can't decode JSON body", errorResponse.Error.Message)
	assert.NotNil(t, errorResponse.Error.RequestID)
}

// Test ParamsErrorHandlerFunc
func TestParamsErrorHandlerFunc(t *testing.T) {
	handler := handlers.ParamsErrorHandler()

	tests := []struct {
		name            string
		err             error
		expectedStatus  int
		expectedMessage string
	}{
		{
			name: "InvalidParamFormatError",
			err: &cmkapi.InvalidParamFormatError{
				ParamName: "test-param",
				Err:       errInvalidFormat,
			},
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Invalid format for parameter test-param: param in invalid format",
		},
		{
			name:            "RequiredHeaderError",
			err:             &cmkapi.RequiredHeaderError{ParamName: "Authorization"},
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Authorization is required, but not found",
		},
		{
			name:            "TooManyValuesForParamError",
			err:             &cmkapi.TooManyValuesForParamError{ParamName: "test-param"},
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Expected one value for test-param, got 0",
		},
		{
			name:            "Default Error",
			err:             errUnknown,
			expectedStatus:  http.StatusInternalServerError,
			expectedMessage: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/test-url", nil)

			handler(recorder, request, tt.err)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			errorResponse := decodeResponse(t, recorder)
			if tt.expectedMessage != "Internal server error" {
				assert.Contains(t, errorResponse.Error.Message, tt.expectedMessage)
			} else {
				assert.Equal(t, tt.expectedMessage, errorResponse.Error.Message)
			}

			assert.NotEmpty(t, *errorResponse.Error.RequestID)
		})
	}
}

// Test ResponseErrorHandlerFunc
func TestResponseErrorHandlerFunc(t *testing.T) {
	handler := handlers.ResponseErrorHandlerFunc()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test-url", nil)

	handler(recorder, request, errFailedToEncodeResponse)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	errorResponse := decodeResponse(t, recorder)
	assert.Equal(t, "Internal server error", errorResponse.Error.Message)
	assert.NotNil(t, errorResponse.Error.RequestID)
}

// Test ErrorResponse function
func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()

	ctx := cmkcontext.InjectRequestID(t.Context())
	requestID, _ := cmkcontext.GetRequestID(ctx)

	errorMessage := cmkapi.ErrorMessage{
		Error: cmkapi.DetailedError{
			RequestID: &requestID,
			Message:   "Test error message",
			Status:    http.StatusInternalServerError,
		},
	}

	write.ErrorResponse(ctx, w, errorMessage)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(errorMessage)
	assert.NoError(t, err)

	assert.JSONEq(t, buf.String(), w.Body.String())
}
