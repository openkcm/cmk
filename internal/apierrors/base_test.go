package apierrors_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/apierrors"
)

func TestInternalServerErrorMessage(t *testing.T) {
	expected := &apierrors.APIError{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Internal server error",
		Status:  http.StatusInternalServerError,
	}
	result := apierrors.InternalServerErrorMessage()
	assert.Equal(t, expected, result)
}

func TestJSONDecodeErrorMessage(t *testing.T) {
	expected := &apierrors.APIError{
		Message: "Can't decode JSON body",
		Code:    "JSON_DECODE_ERROR",
		Status:  http.StatusBadRequest,
	}
	result := apierrors.JSONDecodeErrorMessage()
	assert.Equal(t, expected, result)
}

func TestOAPIValidationErrorMessage(t *testing.T) {
	t.Run("Should bad request", func(t *testing.T) {
		message := "Invalid input"
		code := http.StatusBadRequest
		expected := &apierrors.APIError{
			Code:    "VALIDATION_ERROR",
			Message: message,
			Status:  code,
		}
		result := apierrors.OAPIValidatorErrorMessage(message, code)
		assert.Equal(t, expected, result)
	})

	t.Run("Should Internal Server Error", func(t *testing.T) {
		expected := apierrors.InternalServerErrorMessage()
		result := apierrors.OAPIValidatorErrorMessage("Unxpected Error", http.StatusVariantAlsoNegotiates)
		assert.Equal(t, expected, result)
	})
}

func TestParamsError(t *testing.T) {
	message := "Missing parameters"
	expected := &apierrors.APIError{
		Code:    "PARAMS_ERROR",
		Message: message,
		Status:  http.StatusBadRequest,
	}
	result := apierrors.TooManyParameters(message)
	assert.Equal(t, expected, result)
}

func TestHeaderError(t *testing.T) {
	message := "Invalid headers"
	expected := &apierrors.APIError{
		Code:    "REQUIRED_HEADER_ERROR",
		Message: message,
		Status:  http.StatusBadRequest,
	}
	result := apierrors.RequiredHeaderError(message)
	assert.Equal(t, expected, result)
}
