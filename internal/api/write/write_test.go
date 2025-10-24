package write_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/write"
	"github.com/openkcm/cmk-core/internal/testutils"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
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
