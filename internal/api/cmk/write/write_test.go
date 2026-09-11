package write_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/api/cmk/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestWriteErrorResponse(t *testing.T) {
	t.Run("should write error", func(t *testing.T) {
		ctx := cmkcontext.InjectRequestID(t.Context(), uuid.NewString())
		w := httptest.NewRecorder()

		errorResponse := &apierrors.APIError{
			Code:    "TEST_ERROR",
			Message: "This is a test error",
			Status:  http.StatusBadRequest,
		}

		write.ErrorResponse(ctx, w, errorResponse)

		requestID, _ := cmkcontext.GetRequestID(ctx)

		err := testutils.GetJSONBody[cmkapi.ErrorMessage](t, w)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, requestID, *err.Error.RequestID)
	})
}
