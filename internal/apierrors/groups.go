package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
)

var groups = []APIErrors{
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
