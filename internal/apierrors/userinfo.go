package apierrors

import (
	"net/http"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

var userinfo = []APIErrors{
	{
		Errors: []error{cmkcontext.ErrExtractClientData},
		ExposedError: cmkapi.DetailedError{
			Code:    "UNAUTHORIZED",
			Message: "Unauthorized (Missing client data)",
			Status:  http.StatusUnauthorized,
		},
	},
}
