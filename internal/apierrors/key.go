package apierrors

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

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
	ErrClientDataInvalid                    = errors.New("client data invalid")
)

var key = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrNonEditableCryptoRegionUpdate},
		ExposedError: &APIError{
			Code:    "FORBIDDEN_KEY_ACCESS_UPDATE",
			Message: "Crypto region is not editable",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrBadCryptoRegionData},
		ExposedError: &APIError{
			Code:    "BAD_CRYPTO_DETAILS",
			Message: "Crypto details invalid",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCryptoRegionNotExists},
		ExposedError: &APIError{
			Code:    "FORBIDDEN_KEY_UPDATE",
			Message: "Crypto region does not exist",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrManagementDetailsUpdate},
		ExposedError: &APIError{
			Code:    "FORBIDDEN_KEY_ACCESS_UPDATE",
			Message: "Management details cannot be updated",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrDeleteKey, manager.ErrConnectedSystemToKeyConfig},
		ExposedError: &APIError{
			Code:    "INVALID_KEY_DELETE",
			Message: "Primary key cannot be deleted when systems are still connected",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetKeyDB},
		ExposedError: &APIError{
			Code:    "KEY_ID",
			Message: "Failed to get Key by KeyID",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrPrimaryKeyUnmark},
		ExposedError: &APIError{
			Code:    "PRIMARY_KEY_UNMARK",
			Message: "Primary key cannot be unmarked primary",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrGetKeyDB, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_ID",
			Message: "Key by KeyID not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyToAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY",
			Message: "Failed to transform key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateKey},
		ExposedError: &APIError{
			Code:    "CREATE_KEY",
			Message: "Failed to create key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateKey, manager.ErrCreateKeyDB, repo.ErrUniqueConstraint},
		ExposedError: &APIError{
			Code:    UniqueError,
			Message: "Resource with such ID already exists",
			Status:  http.StatusConflict,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateKey, manager.ErrKeyRegistration, manager.ErrGRPCHYOKAuthFailed},
		ExposedError: &APIError{
			Code:    "REGISTER_KEY_AUTHENTICATION_FAILED",
			Message: "Failed to authenticate with the keystore provider",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: errs.GetGRPCErrorContext,
	},
	{
		InternalErrorChain: []error{ErrCreateKey, manager.ErrKeyRegistration, manager.ErrHYOKProviderKeyNotFound},
		ExposedError: &APIError{
			Code:    "REGISTER_KEY_PROVIDER_KEY_NOT_FOUND",
			Message: "Key not found in the keystore provider",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrCreateKey, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "KeyConfiguration not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrUpdateKey},
		ExposedError: &APIError{
			Code:    "UPDATE_KEY",
			Message: "Failed to update key",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrDeleteKey},
		ExposedError: &APIError{
			Code:    "DELETE_KEY",
			Message: "Failed to delete key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrQueryKeyList},
		ExposedError: &APIError{
			Code:    "QUERY_KEY_LIST",
			Message: "Failed to query key list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrQueryKeyList, gorm.ErrRecordNotFound},
		ExposedError: &APIError{
			Code:    "KEY_CONFIGURATION_NOT_FOUND",
			Message: "KeyConfiguration not found",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, ErrTypeFieldMissingProperty},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: type",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, keyshared.ErrAlgorithmIsRequired},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: algorithm",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, keyshared.ErrRegionIsRequired},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: region",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, transform.ErrAPIUnexpectedProperty},
		ExposedError: &APIError{
			Code:    "UNEXPECTED_PROPERTY",
			Message: "Property is unexpected",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, ErrNameFieldMissingProperty},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: name",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, ErrKeyConfigurationFieldMissingProperty},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: keyConfigurationID",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, keyshared.ErrInvalidKeyProvider},
		ExposedError: &APIError{
			Code:    "NOT_SUPPORTED",
			Message: "Provider is not supported",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY_FROM_API",
			Message: "Failed to transform key from API",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, transformer.ErrGRPCInvalidAccessData},
		ExposedError: &APIError{
			Code:    "INVALID_ACCESS_DATA",
			Message: "Invalid access data provided",
			Status:  http.StatusBadRequest,
		},
		ContextGetter: errs.GetGRPCErrorContext,
	},
	{
		InternalErrorChain: []error{manager.ErrKeyIsNotEnabled},
		ExposedError: &APIError{
			Code:    "KEY_IS_NOT_ENABLED",
			Message: "key is not enabled",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrSetPrimaryKey},
		ExposedError: &APIError{
			Code:    "SET_PRIMARY_KEY",
			Message: "Failed to set primary key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdatePrimary},
		ExposedError: &APIError{
			Code:    "SET_PRIMARY_KEY",
			Message: "Failed to set primary key",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, hyokkey.ErrAccessDetailsIsRequired},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: accessDetails",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, hyokkey.ErrNativeIDIsRequired},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: nativeId",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyFromAPI, keyshared.ErrProviderIsRequired},
		ExposedError: &APIError{
			Code:    "MISSING_PROPERTY",
			Message: "Field is missing: provider",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidKeyTypeForImportParams},
		ExposedError: &APIError{
			Code:    "INVALID_ACTION_FOR_KEY_TYPE",
			Message: "The action cannot be performed for the key type. Only BYOK keys can get import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidKeyTypeForImportKeyMaterial},
		ExposedError: &APIError{
			Code:    "INVALID_ACTION_FOR_KEY_TYPE",
			Message: "The action cannot be performed for the key type. Only BYOK keys can import key material.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidKeyStateForImportParams},
		ExposedError: &APIError{
			Code:    "INVALID_KEY_STATE",
			Message: "Key must be in PENDING_IMPORT state to get import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidKeyStateForImportKeyMaterial},
		ExposedError: &APIError{
			Code:    "INVALID_KEY_STATE",
			Message: "Key must be in PENDING_IMPORT state to import key material.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrEmptyKeyMaterial},
		ExposedError: &APIError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Key material cannot be empty.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrInvalidBase64KeyMaterial},
		ExposedError: &APIError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Key material must be base64 encoded.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrMissingOrExpiredImportParams},
		ExposedError: &APIError{
			Code:    "IMPORT_KEY_MATERIAL",
			Message: "Import parameters missing or expired. Please request new import parameters.",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrDefaultKeystoreNotFound},
		ExposedError: &APIError{
			Code:    "DEFAULT_KEYSTORE_NOT_FOUND",
			Message: "Default keystore not found",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{
			manager.ErrFailedToInitProvider,
			manager.ErrPoolIsDrained,
			manager.ErrGetKeystoreFromPool,
		},
		ExposedError: &APIError{
			Code:    "KEYSTORE_POOL_DRAINED",
			Message: "All keystores in the pool are unavailable",
			Status:  http.StatusServiceUnavailable,
		},
	},
	{
		InternalErrorChain: []error{ErrClientDataInvalid},
		ExposedError: &APIError{
			Code:    "INVALID_CLIENT_DATA",
			Message: "The client data is invalid",
			Status:  http.StatusBadRequest,
		},
	},
}
