package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo"
)

var (
	ErrTransformKeyConfigurationList    = errors.New("failed to transform key configuration list")
	ErrTransformKeyConfigurationFromAPI = errors.New(
		"failed to transform key configuration from API",
	)
	ErrTransformKeyConfigurationToAPI = errors.New("failed to transform key configuration to API")
	ErrGettingKeyConfig               = errors.New("failed to get key configurations")
	ErrGetClientCertificates          = errors.New("failed to get client certificates")
)

var keyConfiguration = []APIErrors{
	{
		Errors: []error{manager.ErrInvalidKeyAdminGroup},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_KEY_CONFIG_ADMIN_GROUP",
			Message: "invalid keyconfig admin group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrConnectedSystemToKeyConfig},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_KEY_CONFIG_DELETE",
			Message: "failed to delete keyconfig with connected systems",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrGettingKeyConfigByID},
		ExposedError: cmkapi.DetailedError{
			Code:    "GETTING_KEY_CONFIG_BY_ID",
			Message: "failed to get key config by ID",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrQueryKeyConfigurationList},
		ExposedError: cmkapi.DetailedError{
			Code:    "QUERY_KEY_CONFIGURATION_LIST",
			Message: "failed to query key configuration list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformKeyConfigurationList},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY_CONFIGURATION_LIST",
			Message: "failed to transform key configuration list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformKeyConfigurationFromAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY_CONFIGURATION_FROM_API",
			Message: "failed to transform key configuration",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrDeleteKeyConfiguration},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_KEY_CONFIGURATION",
			Message: "failed to delete key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrNameCannotBeEmpty},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_PROPERTY",
			Message: "name field cannot be empty",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrUpdateKeyConfiguration},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_KEY_CONFIGURATION",
			Message: "failed to update key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateKeyConfiguration, repo.ErrUniqueConstraint},
		ExposedError: cmkapi.DetailedError{
			Code:    UniqueError,
			Message: "failed to update key configuration because of unique constraint",
			Status:  http.StatusConflict,
		},
	},
	{
		Errors: []error{manager.ErrCreateKeyConfiguration},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_KEY_CONFIGURATION",
			Message: "failed to create key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrCreateKeyConfiguration, repo.ErrUniqueConstraint},
		ExposedError: cmkapi.DetailedError{
			Code:    UniqueError,
			Message: "failed to create key configuration because of unique constraint",
			Status:  http.StatusConflict,
		},
	},
	{
		Errors: []error{ErrGetClientCertificates},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_CLIENT_CERTIFICATES",
			Message: "Failed to get client certificates",
			Status:  http.StatusInternalServerError,
		},
	},
}
