package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrGetDefaultKeystore = errors.New("failed to get default keystore")
	ErrGetWorkflowConfig  = errors.New("failed to get workflow config")
	ErrSetWorkflowConfig  = errors.New("failed to set workflow config")
)

var tenantconfig = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{ErrGetDefaultKeystore},
		ExposedError: &APIError{
			Code:    "GET_DEFAULT_KEYSTORE",
			Message: "Failed to get default keystore",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrGetWorkflowConfig},
		ExposedError: &APIError{
			Code:    "GET_WORKFLOW_CONFIG",
			Message: "Failed to get workflow configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrSetWorkflowConfig},
		ExposedError: &APIError{
			Code:    "SET_WORKFLOW_CONFIG",
			Message: "Failed to update workflow configuration",
			Status:  http.StatusInternalServerError,
		},
	},
}
