package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	registryClient "github.com/openkcm/cmk-core/internal/clients/registry/systems"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/repo"
)

var (
	ErrKeyConfigurationIDRequired = errors.New("KeyConfigurationID is required")
	ErrTransformSystemList        = errors.New("failed to transform system list")
	ErrTransformSystem            = errors.New("failed to transform system")
	ErrTransformSystemFromAPI     = errors.New("failed to transform system from API")
	ErrTransformSystemToAPI       = errors.New("failed to transform system to API")
)

var system = []APIErrors{
	{
		Errors: []error{repo.ErrKeyConfigName, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GETTING_SYSTEM_KEYCONFIG_NAME",
			Message: "failed to get system key config name",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrKeyConfigurationNotFound, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "fail to get system by KeyConfigurationID",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrKeyConfigurationNotFound, repo.ErrGetResource},
		ExposedError: cmkapi.DetailedError{
			Code:    "GETTING_SYSTEM_BY_KEY_CONFIGURATION",
			Message: "fail to get system by KeyConfigurationID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrQuerySystemList},
		ExposedError: cmkapi.DetailedError{
			Code:    "QUERY_SYSTEM_LIST",
			Message: "failed to query system list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformSystemList},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_SYSTEM_LIST",
			Message: "failed to transform system list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformSystem},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_SYSTEM",
			Message: "failed to transform system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformSystemFromAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_SYSTEM_FROM_API",
			Message: "failed to transform system from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformSystemToAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_SYSTEM_TO_API",
			Message: "failed to transform system to API",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystemByID, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_SYSTEM_BY_ID",
			Message: "failed to get system by ID",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrAddSystemNoPrimaryKey},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_TARGET_STATE",
			Message: "system cannot be added to a key configuration without an enabled primary key",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystemByID, repo.ErrGetResource},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_SYSTEM_BY_ID",
			Message: "failed to get system by ID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystem, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_SYSTEM",
			Message: "failed to get system",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystem, repo.ErrGetResource},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_SYSTEM",
			Message: "failed to get system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateSystem},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_SYSTEM",
			Message: "failed to update system",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateSystem, manager.ErrGettingKeyConfigByID},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_KEY_CONFIG_BY_ID",
			Message: "failed to update system: failed to get key configuration by ID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdateSystem, manager.ErrSystemNotLinked},
		ExposedError: cmkapi.DetailedError{
			Code:    "SYSTEM_NOT_LINKED",
			Message: "system is not linked to any key configuration",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, manager.ErrUpdateSystemNoRegClient},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM_NO_CLIENT",
			Message: "error updating key claim for system: registry client not registered",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrSystemsClientDoesNotExist},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM_NO_CLIENT",
			Message: "error updating key claim for system: registry client not registered",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrClientInternalError},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM_REGISTRY_ERROR",
			Message: "error updating key claim for system: registry service internal error",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrKeyClaimAlreadyActive},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM",
			Message: "error updating key claim for system: key claim already set",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrKeyClaimAlreadyInactive},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM",
			Message: "error updating key claim for system: key claim already unset",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrSystemNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM",
			Message: "error updating key claim for system: system not found in registry",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrSettingKeyClaim, registryClient.ErrSystemIsNotLinkedToTenant},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATING_KEY_CLAIM",
			Message: "error updating key claim for system: system is not linked to tenant",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrKeyConfigurationIDNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_CONFIGURATION_ID_NOT_FOUND",
			Message: "Key configuration ID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrKeyConfigurationIDRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_CONFIGURATION_ID_REQUIRED",
			Message: "Key configuration ID is required",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystemLinkByID, repo.ErrNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "GETTING_SYSTEM_LINK_BY_ID",
			Message: "failed to get system link by ID",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrGettingSystemLinkByID, repo.ErrGetResource},
		ExposedError: cmkapi.DetailedError{
			Code:    "GETTING_SYSTEM_LINK_BY_ID",
			Message: "failed to get system link by ID",
			Status:  http.StatusInternalServerError,
		},
	},
}
