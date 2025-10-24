package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/manager"
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

var label = []APIErrors{
	{
		Errors: []error{ErrGetKeyLabels, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_NOT_FOUND",
			Message: "Label by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrLabelNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "LABEL_NOT_FOUND",
			Message: "Label not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrQueryLabelList},
		ExposedError: cmkapi.DetailedError{
			Code:    "QUERY_LABEL_LIST",
			Message: "Failed to query label list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformLabelList},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_LABEL_LIST",
			Message: "Failed to transform label list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrEmptyInputLabel},
		ExposedError: cmkapi.DetailedError{
			Code:    "EMPTY_INPUT_LABEL",
			Message: "Invalid input empty label name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrEmptyInputLabelDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "EMPTY_INPUT_LABEL",
			Message: "Invalid input empty label name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrFetchLabel},
		ExposedError: cmkapi.DetailedError{
			Code:    "FETCH_LABEL",
			Message: "Error fetching label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrInsertLabel},
		ExposedError: cmkapi.DetailedError{
			Code:    "INSERT_LABEL",
			Message: "Error inserting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateLabelDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_LABEL",
			Message: "Error updating label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrUpdateLabel},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_LABEL",
			Message: "Error updating label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrUpdateLabel, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_NOT_FOUND",
			Message: "Update Label by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrDeleteLabel},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_LABEL",
			Message: "Error deleting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrDeleteLabelDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_LABEL",
			Message: "Error deleting label",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformLabelFromAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_LABEL_FROM_API",
			Message: "Failed to transform label from API",
			Status:  http.StatusBadRequest,
		},
	},
}
