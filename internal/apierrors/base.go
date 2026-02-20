package apierrors

import (
	"net/http"
)

const (
	InternalServerErr = "INTERNAL_SERVER_ERROR"
	JSONDecodeErr     = "JSON_DECODE_ERROR"
	ValidationErr     = "VALIDATION_ERROR"
	UnauthorizedErr   = "UNAUTHORIZED"
	ParamsErr         = "PARAMS_ERROR"
	RequiredHeaderErr = "REQUIRED_HEADER_ERROR"
	RequiredParamErr  = "REQUIRED_PARAM_ERROR"
	ForbiddenErr      = "FORBIDDEN"
)

func InternalServerErrorMessage() *APIError {
	return &APIError{
		Code:    InternalServerErr,
		Message: "Internal server error",
		Status:  http.StatusInternalServerError,
	}
}

func JSONDecodeErrorMessage() *APIError {
	return &APIError{
		Code:    JSONDecodeErr,
		Message: "Can't decode JSON body",
		Status:  http.StatusBadRequest,
	}
}

func OAPIValidatorErrorMessage(message string, code int) *APIError {
	switch code {
	case http.StatusBadRequest:
		return &APIError{
			Code:    ValidationErr,
			Message: message,
			Status:  code,
		}
	case http.StatusForbidden:
		return &APIError{
			Code:    ForbiddenErr,
			Message: message,
			Status:  code,
		}
	}

	return InternalServerErrorMessage()
}

func TooManyParameters(message string) *APIError {
	return &APIError{
		Code:    ParamsErr,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

func RequiredHeaderError(message string) *APIError {
	return &APIError{
		Code:    RequiredHeaderErr,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

func RequiredParamError(message string) *APIError {
	return &APIError{
		Code:    RequiredParamErr,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}
