package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
)

const (
	InternalServerErr = "INTERNAL_SERVER_ERROR"
	JSONDecodeErr     = "JSON_DECODE_ERROR"
	ValidationErr     = "VALIDATION_ERROR"
	UnauthorizedErr   = "UNAUTHORIZED"
	ParamsErr         = "PARAMS_ERROR"
	RequiredHeaderErr = "REQUIRED_HEADER_ERROR"
)

func InternalServerErrorMessage() cmkapi.ErrorMessage {
	return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
		Code:    InternalServerErr,
		Message: "Internal server error",
		Status:  http.StatusInternalServerError,
	}}
}

func JSONDecodeErrorMessage() cmkapi.ErrorMessage {
	return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
		Code:    JSONDecodeErr,
		Message: "Can't decode JSON body",
		Status:  http.StatusBadRequest,
	}}
}

func OAPIValidatorErrorMessage(message string, code int) cmkapi.ErrorMessage {
	switch code {
	case http.StatusBadRequest:
		return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
			Code:    ValidationErr,
			Message: message,
			Status:  code,
		}}
	case http.StatusUnauthorized:
		return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
			Code:    UnauthorizedErr,
			Message: message,
			Status:  code,
		}}
	}

	return InternalServerErrorMessage()
}

func TooManyParameters(message string) cmkapi.ErrorMessage {
	return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
		Code:    ParamsErr,
		Message: message,
		Status:  http.StatusBadRequest,
	}}
}

func RequiredHeaderError(message string) cmkapi.ErrorMessage {
	return cmkapi.ErrorMessage{Error: cmkapi.DetailedError{
		Code:    RequiredHeaderErr,
		Message: message,
		Status:  http.StatusBadRequest,
	}}
}

type JoinedError interface {
	Unwrap() []error
}
