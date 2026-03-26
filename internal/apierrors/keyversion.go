package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
)

var (
	ErrQueryKeyVersionList       = errors.New("failed to query key version list")
	ErrTransformKeyVersionList   = errors.New("failed to transform key version list")
	ErrTransformKeyVersionToAPI  = errors.New("failed to transform key version")
	ErrGettingKeyVersionByNumber = errors.New("failed to get key version by number")
)

var keyVersion = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrUpdateKeyVersionDisabled},
		ExposedError: &APIError{
			Code:    "KEY_DISABLED",
			Message: "key must be enabled before attempting to update version",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrNoBodyForCustomerHeldDB},
		ExposedError: &APIError{
			Code:    "NO_BODY_FOR_CUSTOMER_HELD",
			Message: "body must be provided for customer held key rotation",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{ErrQueryKeyVersionList},
		ExposedError: &APIError{
			Code:    "QUERY_KEY_VERSION_LIST",
			Message: "failed to query key version list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyVersionList},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY_VERSION_LIST",
			Message: "failed to transform key version list",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrCreateKeyVersionDB},
		ExposedError: &APIError{
			Code:    "CREATE_KEY_VERSION",
			Message: "failed to create key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformKeyVersionToAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_KEY_VERSION",
			Message: "failed to transform key version",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrGettingKeyVersionByNumber},
		ExposedError: &APIError{
			Code:    "GET_KEY_VERSION_NUMBER",
			Message: "failed to get key version by number",
			Status:  http.StatusNotFound,
		},
	},
	{
		InternalErrorChain: []error{manager.ErrUpdateKeyVersionDB},
		ExposedError: &APIError{
			Code:    "UPDATE_KEY_VERSION",
			Message: "failed to update key version",
			Status:  http.StatusInternalServerError,
		},
	},
}
