package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var ErrNoClientData = cmkapi.DetailedError{
	Code:    "NO_CLIENT_DATA",
	Message: "Missing client data",
	Status:  http.StatusInternalServerError,
}

var userinfo = []APIErrors{
	{
		Errors:       []error{cmkcontext.ErrExtractClientData},
		ExposedError: ErrNoClientData,
	},
}
