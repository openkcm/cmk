package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var ErrNoClientData = &APIError{
	Code:    "NO_CLIENT_DATA",
	Message: "Missing client data",
	Status:  http.StatusInternalServerError,
}

var userinfo = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{cmkcontext.ErrExtractClientData},
		ExposedError:       ErrNoClientData,
	},
}
