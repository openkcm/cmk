package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/manager"
)

var (
	ErrKeyIDPath                 = errors.New("keyID path is invalid")
	ErrQueryKeyVersionList       = errors.New("failed to query key version list")
	ErrTransformKeyVersionList   = errors.New("failed to transform key version list")
	ErrTransformKeyVersionToAPI  = errors.New("failed to transform key version")
	ErrGettingKeyVersionByNumber = errors.New("failed to get key version by number")
	ErrKeyVersionUpdateWrongBody = errors.New("wrong body")
	ErrCreateKeyVersion          = errors.New("failed to create key version")
	ErrUpdateKeyVersion          = errors.New("failed to update key version")
)

var keyVersion = []APIErrors{
	{
		Errors: []error{manager.ErrUpdateKeyVersionDisabled},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_DISABLED",
			Message: "key must be enabled before attempting to update version",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{manager.ErrNoBodyForCustomerHeldDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "NO_BODY_FOR_CUSTOMER_HELD",
			Message: "body must be provided for customer held key rotation",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrCreateKeyVersion, manager.ErrNoBodyForCustomerHeldDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "NO_BODY_FOR_CUSTOMER_HELD",
			Message: "body must be provided for customer held key rotation",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrCreateKeyVersion, manager.ErrBodyForNoCustomerHeldDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "BODY_FOR_NO_CUSTOMER_HELD",
			Message: "body must be provided only for customer held key rotation",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrKeyIDPath},
		ExposedError: cmkapi.DetailedError{
			Code:    "KEY_ID_PATH",
			Message: "keyID path is invalid",
			Status:  http.StatusBadRequest,
		},
	},
	{
		Errors: []error{ErrQueryKeyVersionList},
		ExposedError: cmkapi.DetailedError{
			Code:    "QUERY_KEY_VERSION_LIST",
			Message: "failed to query key version list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformKeyVersionList},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY_VERSION_LIST",
			Message: "failed to transform key version list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{manager.ErrCreateKeyVersionDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_KEY_VERSION",
			Message: "failed to create key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrCreateKeyVersion},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_KEY_VERSION",
			Message: "failed to create key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformKeyVersionToAPI},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_KEY_VERSION",
			Message: "failed to transform key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrGettingKeyVersionByNumber},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_KEY_VERSION_NUMBER",
			Message: "failed to get key version by number",
			Status:  http.StatusNotFound,
		},
	},
	{
		Errors: []error{manager.ErrUpdateKeyVersionDB},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_KEY_VERSION",
			Message: "failed to update key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrUpdateKeyVersion},
		ExposedError: cmkapi.DetailedError{
			Code:    "UPDATE_KEY_VERSION",
			Message: "failed to update key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrKeyVersionUpdateWrongBody},
		ExposedError: cmkapi.DetailedError{
			Code:    "WRONG_BODY",
			Message: "wrong body",
			Status:  http.StatusBadRequest,
		},
	},
}
