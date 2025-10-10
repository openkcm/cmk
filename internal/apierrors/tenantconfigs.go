package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

var ErrGetDefaultKeystore = errors.New("failed to get default keystore")

var tenantconfig = []APIErrors{
	{
		Errors: []error{ErrGetDefaultKeystore},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_DEFAULT_KEYSTORE",
			Message: "Failed to get default keystore",
			Status:  http.StatusInternalServerError,
		},
	},
}
