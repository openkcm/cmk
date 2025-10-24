package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo"
	workflowpkg "github.com/openkcm/cmk-core/internal/workflow"
)

var (
	ErrTransformWorkflowFromAPI = errors.New("failed to transform workflow from API")
	ErrTransformWorkflowToAPI   = errors.New("failed to transform workflow to API")
	ErrGetWorkflow              = errors.New("failed to get workflow")
	ErrCreateWorkflow           = errors.New("failed to create workflow")
	ErrAddApprovers             = errors.New("failed to add approvers to workflow")
	ErrWorkflowCannotTransition = errors.New("workflow cannot transition to specified state")
)

var workflow = []APIErrors{
	{
		Errors: []error{manager.ErrGetWorkflowDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_WORKFLOW",
			Message: "failed to get workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrGetWorkflowDB, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_WORKFLOW",
			Message: "failed to get workflow",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrGetWorkflow},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_WORKFLOW",
			Message: "failed to get workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformWorkflowFromAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_WORKFLOW_FROM_API",
			Message: "failed to transform workflow from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformWorkflowToAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_WORKFLOW_TO_API",
			Message: "failed to transform workflow to API",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrCreateWorkflowDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_WORKFLOW",
			Message: "failed to create workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrCreateWorkflow},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_WORKFLOW",
			Message: "failed to create workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrOngoingWorkflowExist},
		ExposedError: cmkapi.DetailedError{
			Code:    "ONGOING_WORKFLOW",
			Message: "ongoing workflow for artifact already exists",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrCheckOngoingWorkflow},
		ExposedError: cmkapi.DetailedError{
			Code:    "CHECK_ONGOING_WORKFLOW",
			Message: "error checking ongoing workflow for artifact",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrWorkflowNotInitial},
		ExposedError: cmkapi.DetailedError{
			Code:    "WORKFLOW_NOT_INITIAL",
			Message: "workflow is not in initial state",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{workflowpkg.ErrInvalidWorkflowState},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_WORKFLOW_STATE",
			Message: "invalid workflow state",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{workflowpkg.ErrWorkflowExecution},
		ExposedError: cmkapi.DetailedError{
			Code:    "WORKFLOW_EXECUTION",
			Message: "error executing workflow action",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{workflowpkg.ErrUpdateWorkflowState},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_WORKFLOW_STATE",
			Message: "error updating workflow state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{workflowpkg.ErrListApprovers},
		ExposedError: cmkapi.DetailedError{
			Code:    "LIST_APPROVERS",
			Message: "error listing approvers",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{workflowpkg.ErrCheckApprovers},
		ExposedError: cmkapi.DetailedError{
			Code:    "CHECK_APPROVERS",
			Message: "error checking approvers",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateApproverDecision},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_APPROVER_DECISION",
			Message: "error updating approver decision",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{workflowpkg.ErrCheckApproverDecision},
		ExposedError: cmkapi.DetailedError{
			Code:    "CHECK_APPROVER_DECISION",
			Message: "error checking approver decision",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{workflowpkg.ErrInvalidEventActor},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_EVENT_ACTOR",
			Message: "invalid event actor",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{workflowpkg.ErrInsufficientApproverCount},
		ExposedError: cmkapi.DetailedError{
			Code:    "INSUFFICIENT_APPROVER_COUNT",
			Message: "insufficient approvers to transition to next state",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{workflowpkg.ErrTransitionExecution},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSITION_EXECUTION",
			Message: "failed to execute transition",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{workflowpkg.ErrAutomatedTransition},
		ExposedError: cmkapi.DetailedError{
			Code:    "AUTOMATED_TRANSITION",
			Message: "automated transition cannot be triggered by user input",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrInDBTransaction},
		ExposedError: cmkapi.DetailedError{
			Code:    "INDB_TRANSACTION",
			Message: "error when executing sequence of operations in a transaction",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrWorkflowCannotTransitionDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "WORKFLOW_CANNOT_TRANSITION",
			Message: "workflow cannot transition to specified state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrWorkflowCannotTransition},
		ExposedError: cmkapi.DetailedError{
			Code:    "WORKFLOW_CANNOT_TRANSITION",
			Message: "workflow cannot transition to specified state",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrValidateActor},
		ExposedError: cmkapi.DetailedError{
			Code:    "VALIDATE_ACTOR",
			Message: "error validating actor for workflow transition",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrAddApprovers},
		ExposedError: cmkapi.DetailedError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrAddApproversDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrAddApproversDB, manager.ErrApplyTransition},
		ExposedError: cmkapi.DetailedError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrApplyTransition},
		ExposedError: cmkapi.DetailedError{
			Code:    "APPLY_TRANSITION",
			Message: "error applying transition to workflow",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrAddApproversDB, repo.ErrUniqueConstraint},
		ExposedError: cmkapi.DetailedError{
			Code:    "ADD_APPROVERS",
			Message: "error adding approvers to workflow",
			Status:  http.StatusConflict,
		},
	},
	{
		Errors: []error{workflowpkg.ErrListApprovers, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_APPROVERS",
			Message: "failed to get approvers",
			Status:  http.StatusNotFound,
		},
	},
}
