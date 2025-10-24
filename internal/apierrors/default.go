package apierrors

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
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
)

var defaultMapper = []APIErrors{
	{
		Errors: []error{sql.ErrNoRows},
		ExposedError: cmkapi.DetailedError{
			Code:    ResourceNotFound,
			Message: "Requested resource not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{repo.ErrUniqueConstraint},
		ExposedError: cmkapi.DetailedError{
			Code:    UniqueError,
			Message: "Resource with such ID already exists",
			Status:  http.StatusConflict,
		},
	},
	{
		Errors: []error{repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    ResourceNotFound,
			Message: "The requested resource was not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{repo.ErrInvalidUUID},
		ExposedError: cmkapi.DetailedError{
			Code:    BadRequest,
			Message: "Invalid uuid provided",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{repo.ErrGetResource},
		ExposedError: cmkapi.DetailedError{
			Code:    GetResource,
			Message: "The requested resource was not found",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrUnknownProperty},
		ExposedError: cmkapi.DetailedError{
			Code:    "UNKNOWN_PROPERTY",
			Message: "Unknown property",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrActionRequireWorkflow},
		ExposedError: cmkapi.DetailedError{
			Code:    "ACTION_REQUIRE_WORKFLOW",
			Message: "Action requires a workflow",
			Status:  http.StatusForbidden,
		},
	},
}
