package middleware

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	md "github.com/oapi-codegen/nethttp-middleware"

	cmkhandlers "github.com/openkcm/cmk/internal/handlers/cmk"
)

// OAPIMiddleware validates a Request against the OpenAPI Spec
func OAPIMiddleware(swagger *openapi3.T) func(next http.Handler) http.Handler {
	return OAPIMiddlewareWithErrorHandler(swagger, cmkhandlers.OAPIValidatorHandler)
}

// OAPIMiddlewareWithErrorHandler validates a Request against the OpenAPI Spec using the provided error handler.
// Also registers a decoder for merge-patch+json
func OAPIMiddlewareWithErrorHandler(
	swagger *openapi3.T,
	errorHandler md.ErrorHandlerWithOpts,
) func(next http.Handler) http.Handler {
	openapi3filter.RegisterBodyDecoder(
		"application/merge-patch+json",
		openapi3filter.JSONBodyDecoder,
	)

	return md.OapiRequestValidatorWithOptions(
		swagger, &md.Options{
			ErrorHandlerWithOpts: errorHandler,
			Options: openapi3filter.Options{
				AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
				IncludeResponseStatus: true,
			},
			SilenceServersWarning: true,
		},
	)
}
