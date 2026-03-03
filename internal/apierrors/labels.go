package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
)

var (
	ErrGetKeyLabels          = errors.New("failed to get key labels")
	ErrTransformLabelList    = errors.New("failed to transform system list")
	ErrTransformLabelFromAPI = errors.New("failed to transform label from API")
	ErrLabelNotFound         = errors.New("label not found")
	ErrEmptyInputLabel       = errors.New("invalid input empty label name")
	ErrUpdateLabel           = errors.New("failed to update label")
	ErrDeleteLabel           = errors.New("failed to delete label")
)

var label = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{ErrGetKeyLabels, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_NOT_FOUND",
			Message: "Label by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrLabelNotFound},
		ExposedError: &APIError{
			Code:    "LABEL_NOT_FOUND",
			Message: "Label not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrQueryLabelList},
		ExposedError: &APIError{
			Code:    "QUERY_LABEL_LIST",
			Message: "Failed to query label list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformLabelList},
		ExposedError: &APIError{
			Code:    "TRANSFORM_LABEL_LIST",
			Message: "Failed to transform label list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrEmptyInputLabel},
		ExposedError: &APIError{
			Code:    "EMPTY_INPUT_LABEL",
			Message: "Invalid input empty label name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrEmptyInputLabelDB},
		ExposedError: &APIError{
			Code:    "EMPTY_INPUT_LABEL",
			Message: "Invalid input empty label name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrFetchLabel},
		ExposedError: &APIError{
			Code:    "FETCH_LABEL",
			Message: "Error fetching label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInsertLabel},
		ExposedError: &APIError{
			Code:    "INSERT_LABEL",
			Message: "Error inserting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateLabelDB},
		ExposedError: &APIError{
			Code:    "UPDATE_LABEL",
			Message: "Error updating label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrUpdateLabel},
		ExposedError: &APIError{
			Code:    "UPDATE_LABEL",
			Message: "Error updating label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrUpdateLabel, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_NOT_FOUND",
			Message: "Update Label by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrDeleteLabel},
		ExposedError: &APIError{
			Code:    "DELETE_LABEL",
			Message: "Error deleting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrDeleteLabelDB},
		ExposedError: &APIError{
			Code:    "DELETE_LABEL",
			Message: "Error deleting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformLabelFromAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_LABEL_FROM_API",
			Message: "Failed to transform label from API",
			Status:  http.StatusBadRequest,
		},
	},
}
