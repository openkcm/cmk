package write

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/log"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
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
