package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/errs"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
)

var (
	ErrKeyConfigurationIDRequired = errors.New("KeyConfigurationID is required")
	ErrTransformSystemList        = errors.New("failed to transform system list")
	ErrTransformSystem            = errors.New("failed to transform system")
	ErrTransformSystemFromAPI     = errors.New("failed to transform system from API")
	ErrTransformSystemToAPI       = errors.New("failed to transform system to API")
)

var system = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{repo.ErrKeyConfigName, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GETTING_SYSTEM_KEYCONFIG_NAME",
			Message: "failed to get system key config name",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{eventprocessor.ErrNoPreviousEvent},
		ExposedError: &APIError{
			Code:    "NO_PREVIOUS_SYSTEM_STATE",
			Message: "failed to cancel action",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrKeyConfigurationNotFound, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "fail to get system by KeyConfigurationID",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrKeyConfigurationNotFound, repo.ErrGetResource},
		ExposedError: &APIError{
			Code:    "GETTING_SYSTEM_BY_KEY_CONFIGURATION",
			Message: "fail to get system by KeyConfigurationID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrQuerySystemList},
		ExposedError: &APIError{
			Code:    "QUERY_SYSTEM_LIST",
			Message: "failed to query system list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformSystemList},
		ExposedError: &APIError{
			Code:    "TRANSFORM_SYSTEM_LIST",
			Message: "failed to transform system list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformSystem},
		ExposedError: &APIError{
			Code:    "TRANSFORM_SYSTEM",
			Message: "failed to transform system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformSystemFromAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_SYSTEM_FROM_API",
			Message: "failed to transform system from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformSystemToAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_SYSTEM_TO_API",
			Message: "failed to transform system to API",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystemByID, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GET_SYSTEM_BY_ID",
			Message: "failed to get system by ID",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrConnectSystemNoPrimaryKey},
		ExposedError: &APIError{
			Code:    "INVALID_TARGET_STATE",
			Message: "system cannot be added to a key configuration without an enabled primary key",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystemByID, repo.ErrGetResource},
		ExposedError: &APIError{
			Code:    "GET_SYSTEM_BY_ID",
			Message: "failed to get system by ID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystem, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GET_SYSTEM",
			Message: "failed to get system",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystem, repo.ErrGetResource},
		ExposedError: &APIError{
			Code:    "GET_SYSTEM",
			Message: "failed to get system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateSystem},
		ExposedError: &APIError{
			Code:    "UPDATE_SYSTEM",
			Message: "failed to update system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateSystem, manager.ErrGettingKeyConfigByID},
		ExposedError: &APIError{
			Code:    "GET_KEY_CONFIG_BY_ID",
			Message: "failed to update system: failed to get key configuration by ID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateSystem, manager.ErrSystemNotLinked},
		ExposedError: &APIError{
			Code:    "SYSTEM_NOT_LINKED",
			Message: "system is not linked to any key configuration",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrKeyConfigurationIDNotFound},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_ID_NOT_FOUND",
			Message: "Key configuration ID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrKeyConfigurationIDRequired},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_ID_REQUIRED",
			Message: "Key configuration ID is required",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystemLinkByID, repo.ErrNotFound},
		ExposedError: &APIError{
			Code:    "GETTING_SYSTEM_LINK_BY_ID",
			Message: "failed to get system link by ID",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGettingSystemLinkByID, repo.ErrGetResource},
		ExposedError: &APIError{
			Code:    "GETTING_SYSTEM_LINK_BY_ID",
			Message: "failed to get system link by ID",
			Status:  http.StatusInternalServerError,
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
}
