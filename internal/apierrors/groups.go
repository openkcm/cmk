package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	MultipleRolesInGroupsCode = "MULTIPLE_ROLES_NOT_ALLOWED"
)

var groups = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrMultipleRolesInGroups},
		ExposedError: &APIError{
			Code:    MultipleRolesInGroupsCode,
			Message: "users with multiple roles are not allowed",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrZeroRolesInGroups},
		ExposedError: &APIError{
			Code:    "ZERO_ROLES_NOT_ALLOWED",
			Message: "users without any roles are not allowed",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateGroups, model.ErrInvalidName},
		ExposedError: &APIError{
			Code:    "INVALID_GROUP_NAME",
			Message: "Invalid name for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateGroups, model.ErrInvalidIAMIdentifier},
		ExposedError: &APIError{
			Code:    "INVALID_GROUP_IAM_IDENTIFIER",
			Message: "Invalid IamIdentifier for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGroupRole},
		ExposedError: &APIError{
			Code:    "UNSUPPORTED_GROUP_ROLE",
			Message: "Unsupported role for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateGroups, repo.ErrCreateResource},
		ExposedError: &APIError{
			Code:    "CREATE_GROUP",
			Message: "Failed to create a group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateGroups},
		ExposedError: &APIError{
			Code:    "CREATE_GROUP",
			Message: "Failed to create a group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidGroupDelete},
		ExposedError: &APIError{
			Code:    "DELETE_INVALID_GROUP",
			Message: "Group cannot be deleted",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidGroupUpdate},
		ExposedError: &APIError{
			Code:    "INVALID_GROUP_UPDATE",
			Message: "Group cannot be updated",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrNameCannotBeEmpty},
		ExposedError: &APIError{
			Code:    "RENAME_INVALID_NAME",
			Message: "Group name cannot be empty",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrDeleteGroups},
		ExposedError: &APIError{
			Code:    "DELETE_GROUP",
			Message: "Failed to delete the group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetGroups},
		ExposedError: &APIError{
			Code:    "GET_GROUP",
			Message: "Failed to get the group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetGroups, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GET_GROUP_NOT_FOUND",
			Message: "Group does not exist",
			Status:  http.StatusNotFound,
		},
	},
}
