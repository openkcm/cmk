package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
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

var keyConfiguration = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrInvalidKeyAdminGroup},
		ExposedError: &APIError{
			Code:    "INVALID_KEY_CONFIG_ADMIN_GROUP",
			Message: "invalid keyconfig admin group",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrConnectedSystemToKeyConfig},
		ExposedError: &APIError{
			Code:    "INVALID_KEY_CONFIG_DELETE",
			Message: "failed to delete keyconfig with connected systems",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingKeyConfigByID},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "Key configuration not found or insufficient access permissions",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrKeyConfigurationNotAllowed},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "Key configuration not found or insufficient access permissions",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrQueryKeyConfigurationList},
		ExposedError: &APIError{
			Code:    "QUERY_KEY_CONFIGURATION_LIST",
			Message: "failed to query key configuration list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyConfigurationList},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY_CONFIGURATION_LIST",
			Message: "failed to transform key configuration list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyConfigurationFromAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY_CONFIGURATION_FROM_API",
			Message: "failed to transform key configuration",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrDeleteKeyConfiguration},
		ExposedError: &APIError{
			Code:    "DELETE_KEY_CONFIGURATION",
			Message: "failed to delete key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrNameCannotBeEmpty},
		ExposedError: &APIError{
			Code:    "INVALID_PROPERTY",
			Message: "name field cannot be empty",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateKeyConfiguration},
		ExposedError: &APIError{
			Code:    "UPDATE_KEY_CONFIGURATION",
			Message: "failed to update key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateKeyConfiguration, repo.ErrUniqueConstraint},
		ExposedError: &APIError{
			Code:    UniqueError,
			Message: "failed to update key configuration because of unique constraint",
			Status:  http.StatusConflict,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateKeyConfiguration},
		ExposedError: &APIError{
			Code:    "CREATE_KEY_CONFIGURATION",
			Message: "failed to create key configuration",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateKeyConfiguration, repo.ErrUniqueConstraint},
		ExposedError: &APIError{
			Code:    UniqueError,
			Message: "failed to create key configuration because of unique constraint",
			Status:  http.StatusConflict,
		},
	},
	{
		InternalErrorChain: []error{ErrGetClientCertificates},
		ExposedError: &APIError{
			Code:    "GET_CLIENT_CERTIFICATES",
			Message: "Failed to get client certificates",
			Status:  http.StatusInternalServerError,
		},
	},
}
