package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	md "github.com/oapi-codegen/nethttp-middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/write"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/log"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

// OAPIValidatorHandler is called when OAPI Required fields are missing from Request
func OAPIValidatorHandler(
	ctx context.Context,
	err error,
	w http.ResponseWriter,
	_ *http.Request,
	opts md.ErrorHandlerOpts,
) {
	log.Error(ctx, "Request does not follow OAPI contract", err)

	write.ErrorResponse(ctx, w, apierrors.OAPIValidatorErrorMessage(err.Error(), opts.StatusCode))
}

// ParamsErrorHandler is called whenever Request doesn't follow OAPI Endpoint Parameters (Path and Query)
// Must create RequestID and logger because middlewares weren't ran
func ParamsErrorHandler() func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		ctx := cmkcontext.InjectRequestID(r.Context())
		requestID, _ := cmkcontext.GetRequestID(ctx)

		ctx = slogctx.With(ctx,
			slog.String("RequestID", requestID),
		)

		log.Error(ctx, "The error encountered during parameters binding", err)

		var errorResponse cmkapi.ErrorMessage

		var (
			invalidFormatErr     *cmkapi.InvalidParamFormatError
			requiredHeaderErr    *cmkapi.RequiredHeaderError
			tooManyParametersErr *cmkapi.TooManyValuesForParamError
			requiredParamErr     *cmkapi.RequiredParamError
		)

		switch {
		case errors.As(err, &invalidFormatErr):
			errorResponse = apierrors.TooManyParameters(err.Error())
		case errors.As(err, &requiredHeaderErr):
			errorResponse = apierrors.RequiredHeaderError(requiredHeaderErr.Error())
		case errors.As(err, &tooManyParametersErr):
			errorResponse = apierrors.TooManyParameters(tooManyParametersErr.Error())
		case errors.As(err, &requiredParamErr):
			errorResponse = apierrors.RequiredParamError(requiredParamErr.Error())
		default:
			errorResponse = apierrors.InternalServerErrorMessage()
		}

		write.ErrorResponse(ctx, w, errorResponse)
	}
}

// RequestErrorHandlerFunc is called when Request JSON Body Decoding fails
func RequestErrorHandlerFunc() func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error(r.Context(), "Receiving Request", err)

		write.ErrorResponse(r.Context(), w, apierrors.JSONDecodeErrorMessage())
	}
}

// ResponseErrorHandlerFunc is called when HTTP Handlers (Controller Functions) return invalid responses
func ResponseErrorHandlerFunc() func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error(r.Context(), "Processing Request", err)

		e := apierrors.TransformToAPIError(r.Context(), err)
		write.ErrorResponse(r.Context(), w, *e)
	}
}
