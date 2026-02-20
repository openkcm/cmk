package write

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// ErrorResponse writes an error response to the client and logs the error
func ErrorResponse(ctx context.Context, w http.ResponseWriter, apiError *apierrors.APIError) {
	requestID, _ := cmkcontext.GetRequestID(ctx)

	errorResponse := cmkapi.ErrorMessage{
		Error: cmkapi.DetailedError{
			Code:      apiError.Code,
			Context:   apiError.Context,
			Message:   apiError.Message,
			RequestID: &requestID,
			Status:    apiError.Status,
		},
	}

	w.WriteHeader(errorResponse.Error.Status)

	enc := json.NewEncoder(w)

	err := enc.Encode(&errorResponse)
	if err != nil {
		log.Error(ctx, "Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)

		return
	}
}
