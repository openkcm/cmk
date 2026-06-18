package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
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
		InternalErrorChain: []error{ErrSetWorkflowConfig, manager.ErrWorkflowEnableDisableNotAllowed},
		ExposedError: &APIError{
			Code:    "INVALID_SETTING",
			Message: "workflow enable/disable is only allowed for TEST tenants",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: func(_ error) map[string]any {
			return map[string]any{"setting": "enabled"}
		},
	},
	{
		InternalErrorChain: []error{ErrSetWorkflowConfig, manager.ErrRetentionLessThanMinimum},
		ExposedError: &APIError{
			Code:    "INVALID_SETTING",
			Message: "retentionPeriodDays must be at least 30",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: func(_ error) map[string]any {
			return map[string]any{"setting": "retentionPeriodDays"}
		},
	},
	{
		InternalErrorChain: []error{ErrSetWorkflowConfig, manager.ErrDefaultExpiryExceedsMax},
		ExposedError: &APIError{
			Code:    "INVALID_SETTING",
			Message: "defaultExpiryPeriodDays must be less than or equal to maxExpiryPeriodDays",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: func(_ error) map[string]any {
			return map[string]any{"setting": "defaultExpiryPeriodDays"}
		},
	},
	{
		InternalErrorChain: []error{ErrSetWorkflowConfig, manager.ErrMinimumApprovalsTooLow},
		ExposedError: &APIError{
			Code:    "INVALID_SETTING",
			Message: "minimumApprovals must be at least 2",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: func(_ error) map[string]any {
			return map[string]any{"setting": "minimumApprovals"}
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
