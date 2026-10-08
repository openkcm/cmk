package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var ErrNoBusinessUserData = &APIError{
	Code:    "NO_BUSINESS_DATA",
	Message: "Missing business data",
	Status:  http.StatusInternalServerError,
}

// ErrInternalRoleNotSupported maps to 500 because an internal role reaching a
// business-user code path is a programming error, not an expected client condition.
var ErrInternalRoleNotSupported = &APIError{
	Code:    "INTERNAL_ROLE_NOT_SUPPORTED",
	Message: "Operation not supported for this internal role",
	Status:  http.StatusInternalServerError,
}

var userinfo = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{cmkcontext.ErrExtractBusinessUserData},
		ExposedError:       ErrNoBusinessUserData,
	},
	{
		InternalErrorChain: []error{manager.ErrInternalRoleNotSupported},
		ExposedError:       ErrInternalRoleNotSupported,
	},
}
