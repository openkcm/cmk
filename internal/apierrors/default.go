package apierrors

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	ResourceNotFound = "RESOURCE_NOT_FOUND"
	UniqueError      = "UNIQUE_ERROR"
	BadRequest       = "BAD_REQUEST"
	GetResource      = "GET_RESOURCE"
)

var (
	ErrActionRequireWorkflow = errors.New("action requires a workflow")
	ErrUnknownProperty       = errors.New("unknown property")
	ErrBadOdataFilter        = errors.New("bad odata filter")
)

var defaultMapper = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{sql.ErrNoRows},
		ExposedError: &APIError{
			Code:    ResourceNotFound,
			Message: "Requested resource not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{repo.ErrUniqueConstraint},
		ExposedError: &APIError{
			Code:    UniqueError,
			Message: "Resource with such ID already exists",
			Status:  http.StatusConflict,
		},
	},
	{
		InternalErrorChain: []error{repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    ResourceNotFound,
			Message: "The requested resource was not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{repo.ErrInvalidUUID},
		ExposedError: &APIError{
			Code:    BadRequest,
			Message: "Invalid uuid provided",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrBadOdataFilter},
		ExposedError: &APIError{
			Code:    BadRequest,
			Message: "Bad Odata filter provided",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{repo.ErrGetResource},
		ExposedError: &APIError{
			Code:    GetResource,
			Message: "The requested resource was not found",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrUnknownProperty},
		ExposedError: &APIError{
			Code:    "UNKNOWN_PROPERTY",
			Message: "Unknown property",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrActionRequireWorkflow},
		ExposedError: &APIError{
			Code:    "ACTION_REQUIRE_WORKFLOW",
			Message: "Action requires a workflow",
			Status:  http.StatusForbidden,
		},
	},
}
