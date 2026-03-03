package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
	workflowpkg "github.com/openkcm/cmk/internal/workflow"
)

var (
	ErrTransformWorkflowFromAPI = errors.New("failed to transform workflow from API")
	ErrTransformWorkflowToAPI   = errors.New("failed to transform workflow to API")
	ErrGetWorkflow              = errors.New("failed to get workflow")
	ErrCreateWorkflow           = errors.New("failed to create workflow")
	ErrAddApprovers             = errors.New("failed to add approvers to workflow")
	ErrWorkflowCannotTransition = errors.New("workflow cannot transition to specified state")
)

var workflow = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrWorkflowCreationNotAllowed},
		ExposedError: &APIError{
			Code:    "FORBIDDEN_WORKFLOW_CREATION",
			Message: "Workflow creation unauthorized (may not be keyadmin or in correct group)",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCheckWorkflow},
		ExposedError: &APIError{
			Code:    "CHECK_WORKFLOW",
			Message: "failed to check artifacts of workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetWorkflowDB},
		ExposedError: &APIError{
			Code:    "GET_WORKFLOW",
			Message: "failed to get workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrWorkflowNotAllowed},
		ExposedError: &APIError{
			Code:    "WORKFLOW_NOT_FOUND",
			Message: "Workflow not found or insufficient access permissions",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetWorkflowDB, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GET_WORKFLOW",
			Message: "Workflow not found or insufficient access permissions",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrGetWorkflow},
		ExposedError: &APIError{
			Code:    "GET_WORKFLOW",
			Message: "failed to get workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformWorkflowFromAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_WORKFLOW_FROM_API",
			Message: "failed to transform workflow from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformWorkflowToAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_WORKFLOW_TO_API",
			Message: "failed to transform workflow to API",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateWorkflowDB},
		ExposedError: &APIError{
			Code:    "CREATE_WORKFLOW",
			Message: "failed to create workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateWorkflow},
		ExposedError: &APIError{
			Code:    "CREATE_WORKFLOW",
			Message: "failed to create workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateWorkflow, manager.ErrOngoingWorkflowExist},
		ExposedError: &APIError{
			Code:    "ONGOING_WORKFLOW",
			Message: "ongoing workflow for artifact already exists",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCheckOngoingWorkflow},
		ExposedError: &APIError{
			Code:    "CHECK_ONGOING_WORKFLOW",
			Message: "error checking ongoing workflow for artifact",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrInvalidWorkflowState},
		ExposedError: &APIError{
			Code:    "INVALID_WORKFLOW_STATE",
			Message: "invalid workflow state",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrWorkflowExecution},
		ExposedError: &APIError{
			Code:    "WORKFLOW_EXECUTION",
			Message: "error executing workflow action",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrUpdateWorkflowState},
		ExposedError: &APIError{
			Code:    "UPDATE_WORKFLOW_STATE",
			Message: "error updating workflow state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrListApprovers},
		ExposedError: &APIError{
			Code:    "LIST_APPROVERS",
			Message: "error listing approvers",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrCheckApprovers},
		ExposedError: &APIError{
			Code:    "CHECK_APPROVERS",
			Message: "error checking approvers",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateApproverDecision},
		ExposedError: &APIError{
			Code:    "UPDATE_APPROVER_DECISION",
			Message: "error updating approver decision",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrCheckApproverDecision},
		ExposedError: &APIError{
			Code:    "CHECK_APPROVER_DECISION",
			Message: "error checking approver decision",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrInvalidEventActor},
		ExposedError: &APIError{
			Code:    "INVALID_EVENT_ACTOR",
			Message: "invalid event actor",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrInsufficientApproverCount},
		ExposedError: &APIError{
			Code:    "INSUFFICIENT_APPROVER_COUNT",
			Message: "insufficient approvers to transition to next state",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrTransitionExecution},
		ExposedError: &APIError{
			Code:    "TRANSITION_EXECUTION",
			Message: "failed to execute transition",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrAutomatedTransition},
		ExposedError: &APIError{
			Code:    "AUTOMATED_TRANSITION",
			Message: "automated transition cannot be triggered by user input",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInDBTransaction},
		ExposedError: &APIError{
			Code:    "INDB_TRANSACTION",
			Message: "error when executing sequence of operations in a transaction",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrWorkflowCannotTransitionDB},
		ExposedError: &APIError{
			Code:    "WORKFLOW_CANNOT_TRANSITION",
			Message: "workflow cannot transition to specified state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrWorkflowCannotTransition},
		ExposedError: &APIError{
			Code:    "WORKFLOW_CANNOT_TRANSITION",
			Message: "workflow cannot transition to specified state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrValidateActor},
		ExposedError: &APIError{
			Code:    "VALIDATE_ACTOR",
			Message: "error validating actor for workflow transition",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrAddApprovers},
		ExposedError: &APIError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrAddApproversDB},
		ExposedError: &APIError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrAddApproversDB, manager.ErrApplyTransition},
		ExposedError: &APIError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrApplyTransition},
		ExposedError: &APIError{
			Code:    "APPLY_TRANSITION",
			Message: "error applying transition to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrAddApproversDB, repo.ErrUniqueConstraint},
		ExposedError: &APIError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusConflict,
		},
	},
	{
		InternalErrorChain: []error{workflowpkg.ErrListApprovers, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GET_APPROVERS",
			Message: "failed to get approvers",
			Status:  http.StatusNotFound,
		},
	},
}
