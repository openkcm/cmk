package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

var ErrGetDefaultKeystore = errors.New("failed to get default keystore")
var ErrGetWorkflowConfig = errors.New("failed to get workflow config")
var ErrSetWorkflowConfig = errors.New("failed to set workflow config")

var tenantconfig = []APIErrors{
	{
		Errors: []error{ErrGetDefaultKeystore},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_DEFAULT_KEYSTORE",
			Message: "Failed to get default keystore",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrGetWorkflowConfig},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_WORKFLOW_CONFIG",
			Message: "Failed to get workflow configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrSetWorkflowConfig},
		ExposedError: cmkapi.DetailedError{
			Code:    "SET_WORKFLOW_CONFIG",
			Message: "Failed to update workflow configuration",
			Status:  http.StatusInternalServerError,
		},
	},
}
