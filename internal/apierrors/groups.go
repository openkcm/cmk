package apierrors

import (
	"net/http"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
)

const (
	MultipleRolesInGroupsCode = "MULTIPLE_ROLES_NOT_ALLOWED"
)

var groups = []APIErrors{
	{
		Errors: []error{manager.ErrMultipleRolesInGroups},
		ExposedError: cmkapi.DetailedError{
			Code:    MultipleRolesInGroupsCode,
			Message: "users with multiple roles are not allowed",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{manager.ErrCreateGroups, model.ErrInvalidName},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_GROUP_NAME",
			Message: "Invalid name for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrCreateGroups, model.ErrInvalidIAMIdentifier},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_GROUP_IAM_IDENTIFIER",
			Message: "Invalid IamIdentifier for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrGroupRole},
		ExposedError: cmkapi.DetailedError{
			Code:    "UNSUPPORTED_GROUP_ROLE",
			Message: "Unsupported role for selected group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrCreateGroups, repo.ErrCreateResource},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_GROUP",
			Message: "Failed to create a group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrCreateGroups},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_GROUP",
			Message: "Failed to create a group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidGroupDelete},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_INVALID_GROUP",
			Message: "Group cannot be deleted",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidGroupRename},
		ExposedError: cmkapi.DetailedError{
			Code:    "RENAME_INVALID_GROUP",
			Message: "Group cannot be renamed",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrNameCannotBeEmpty},
		ExposedError: cmkapi.DetailedError{
			Code:    "RENAME_INVALID_NAME",
			Message: "Group name cannot be empty",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrDeleteGroups},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_GROUP",
			Message: "Failed to delete the group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrGetGroups},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_GROUP",
			Message: "Failed to get the group",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrGetGroups, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_GROUP_NOT_FOUND",
			Message: "Group does not exist",
			Status:  http.StatusNotFound,
		},
	},
}
