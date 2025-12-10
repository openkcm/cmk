package write

import (
	"context"
	"encoding/json"
	"net/http"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/log"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

// ErrorResponse writes an error response to the client and logs the error
func ErrorResponse(ctx context.Context, w http.ResponseWriter, errorResponse cmkapi.ErrorMessage) {
	requestID, _ := cmkcontext.GetRequestID(ctx)

	errorResponse.Error.RequestID = &requestID

	w.WriteHeader(errorResponse.Error.Status)

	enc := json.NewEncoder(w)

	err := enc.Encode(&errorResponse)
	if err != nil {
		log.Error(ctx, "Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)

		return
	}
}
