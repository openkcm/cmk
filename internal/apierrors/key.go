package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/key/hyokkey"
	"github.com/openkcm/cmk/internal/api/transform/key/keyshared"
	"github.com/openkcm/cmk/internal/api/transform/key/transformer"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
)

var (
	ErrTransformKeyToAPI                    = errors.New("failed to transform key to API")
	ErrCreateKey                            = errors.New("failed to create key")
	ErrUpdateKey                            = errors.New("failed to update key")
	ErrDeleteKey                            = errors.New("failed to delete key")
	ErrQueryKeyList                         = errors.New("failed to query key list")
	ErrNameFieldMissingProperty             = errors.New("field is missing name")
	ErrTypeFieldMissingProperty             = errors.New("field is missing type")
	ErrKeyConfigurationFieldMissingProperty = errors.New("field is missing keyConfigurationID")
	ErrTransformKeyFromAPI                  = errors.New("failed to transform key from API")
	ErrSetPrimaryKey                        = errors.New("failed to set primary key")
	ErrDefaultKeystoreNotFound              = errors.New("default keystore not found")
)

var key = []APIErrors{
	{
		Errors: []error{manager.ErrNonEditableCryptoRegionUpdate},
		ExposedError: cmkapi.DetailedError{
			Code:    "FORBIDDEN_KEY_ACCESS_UPDATE",
			Message: "Crypto region is not editable",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{manager.ErrBadCryptoRegionData},
		ExposedError: cmkapi.DetailedError{
			Code:    "BAD_CRYPTO_DETAILS",
			Message: "Crypto details invalid",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrCryptoRegionNotExists},
		ExposedError: cmkapi.DetailedError{
			Code:    "FORBIDDEN_KEY_UPDATE",
			Message: "Crypto region does not exist",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{manager.ErrManagementDetailsUpdate},
		ExposedError: cmkapi.DetailedError{
			Code:    "FORBIDDEN_KEY_ACCESS_UPDATE",
			Message: "Management details cannot be updated",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{manager.ErrDeleteKey, manager.ErrConnectedSystemToKeyConfig},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_KEY_DELETE",
			Message: "Primary key cannot be deleted when systems are still connected",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrGetKeyDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_ID",
			Message: "Failed to get Key by KeyID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrPrimaryKeyUnmark},
		ExposedError: cmkapi.DetailedError{
			Code:    "PRIMARY_KEY_UNMARK",
			Message: "Primary key cannot be unmarked primary",
			Status:  http.StatusForbidden,
		},
	},
	{
		Errors: []error{manager.ErrGetKeyDB, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_ID",
			Message: "Key by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrTransformKeyToAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY",
			Message: "Failed to transform key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrCreateKey},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_KEY",
			Message: "Failed to create key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrCreateKey, manager.ErrCreateKeyDB, repo.ErrUniqueConstraint},
		ExposedError: cmkapi.DetailedError{
			Code:    UniqueError,
			Message: "Resource with such ID already exists",
			Status:  http.StatusConflict,
		},
	},
	{
		Errors: []error{ErrCreateKey, manager.ErrKeyRegistration, manager.ErrGRPCHYOKAuthFailed},
		ExposedError: cmkapi.DetailedError{
			Code:    "REGISTER_KEY_AUTHENTICATION_FAILED",
			Message: "Failed to authenticate with the keystore provider",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: errs.GetGRPCErrorContext,
	},
	{
		Errors: []error{ErrCreateKey, manager.ErrKeyRegistration, manager.ErrHYOKProviderKeyNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "REGISTER_KEY_PROVIDER_KEY_NOT_FOUND",
			Message: "Key not found in the keystore provider",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrCreateKey, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "KeyConfiguration not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrUpdateKey},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_KEY",
			Message: "Failed to update key",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrDeleteKey},
		ExposedError: cmkapi.DetailedError{
			Code:    "DELETE_KEY",
			Message: "Failed to delete key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrQueryKeyList},
		ExposedError: cmkapi.DetailedError{
			Code:    "QUERY_KEY_LIST",
			Message: "Failed to query key list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrQueryKeyList, gorm.ErrRecordNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "KeyConfiguration not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, ErrTypeFieldMissingProperty},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: type",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, keyshared.ErrAlgorithmIsRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: algorithm",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, keyshared.ErrRegionIsRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: region",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, transform.ErrAPIUnexpectedProperty},
		ExposedError: cmkapi.DetailedError{
			Code:    "UNEXPECTED_PROPERTY",
			Message: "Property is unexpected",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, ErrNameFieldMissingProperty},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, ErrKeyConfigurationFieldMissingProperty},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: keyConfigurationID",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, keyshared.ErrInvalidKeyProvider},
		ExposedError: cmkapi.DetailedError{
			Code:    "NOT_SUPPORTED",
			Message: "Provider is not supported",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY_FROM_API",
			Message: "Failed to transform key from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, transformer.ErrGRPCInvalidAccessData},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_ACCESS_DATA",
			Message: "Invalid access data provided",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: errs.GetGRPCErrorContext,
	},
	{
		Errors: []error{manager.ErrKeyIsNotEnabled},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_IS_NOT_ENABLED",
			Message: "key is not enabled",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrSetPrimaryKey},
		ExposedError: cmkapi.DetailedError{
			Code:    "SET_PRIMARY_KEY",
			Message: "Failed to set primary key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrUpdatePrimary},
		ExposedError: cmkapi.DetailedError{
			Code:    "SET_PRIMARY_KEY",
			Message: "Failed to set primary key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, hyokkey.ErrAccessDetailsIsRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: accessDetails",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, hyokkey.ErrNativeIDIsRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: nativeId",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrTransformKeyFromAPI, keyshared.ErrProviderIsRequired},
		ExposedError: cmkapi.DetailedError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: provider",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidKeyTypeForImportParams},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_ACTION_FOR_KEY_TYPE",
			Message: "The action cannot be performed for the key type. Only BYOK keys can get import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidKeyTypeForImportKeyMaterial},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_ACTION_FOR_KEY_TYPE",
			Message: "The action cannot be performed for the key type. Only BYOK keys can import key material.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidKeyStateForImportParams},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_KEY_STATE",
			Message: "Key must be in PENDING_IMPORT state to get import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidKeyStateForImportKeyMaterial},
		ExposedError: cmkapi.DetailedError{
			Code:    "INVALID_KEY_STATE",
			Message: "Key must be in PENDING_IMPORT state to import key material.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrEmptyKeyMaterial},
		ExposedError: cmkapi.DetailedError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Key material cannot be empty.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrInvalidBase64KeyMaterial},
		ExposedError: cmkapi.DetailedError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Key material must be base64 encoded.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrMissingOrExpiredImportParams},
		ExposedError: cmkapi.DetailedError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Import parameters missing or expired. Please request new import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrDefaultKeystoreNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    "DEFAULT_KEYSTORE_NOT_FOUND",
			Message: "Default keystore not found",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrFailedToInitProvider, manager.ErrPoolIsDrained, manager.ErrGetKeystoreFromPool},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEYSTORE_POOL_DRAINED",
			Message: "All keystores in the pool are unavailable",
			Status:  http.StatusServiceUnavailable,
		},
	},
}
