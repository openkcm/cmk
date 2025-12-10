package write_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/write"
	"github.tools.sap/kms/cmk/internal/testutils"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

func TestWriteErrorResponse(t *testing.T) {
	t.Run("should write error", func(t *testing.T) {
		ctx := cmkcontext.InjectRequestID(t.Context())
		w := httptest.NewRecorder()
		errorResponse := cmkapi.ErrorMessage{
			Error: cmkapi.DetailedError{
				Code:    "TEST_ERROR",
				Message: "This is a test error",
				Status:  http.StatusBadRequest,
			},
		}

		write.ErrorResponse(ctx, w, errorResponse)

		requestID, _ := cmkcontext.GetRequestID(ctx)

		err := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, requestID, *err.Error.RequestID)
	})
}
