package daemon

import (
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/authz"
)

type ServeMux struct {
	httpServeMux http.ServeMux
	BaseURL      string
}

type ServeMuxOption func(*ServeMux)

// WithSwaggerUI adds an endpoint to serve Swagger UI at /cmk/v1/swagger
// The Swagger UI will load the OpenAPI spec inline without requiring a separate endpoint
// Note: The {tenant} parameter is removed from the base URL for this endpoint
func WithSwaggerUI(swagger *openapi3.T) ServeMuxOption {
	return func(m *ServeMux) {
		// Remove {tenant} parameter from base URL for swagger endpoint
		swaggerBaseURL := strings.TrimSuffix(m.BaseURL, "/{tenant}")
		pattern := "GET " + swaggerBaseURL + "/swagger"
		m.httpServeMux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			html, err := cmkapi.SwaggerUI(swagger)
			if err != nil {
				e := apierrors.APIErrorMapper.Transform(r.Context(), err)
				write.ErrorResponse(r.Context(), w, e)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, err = w.Write([]byte(html))
			if err != nil {
				e := apierrors.APIErrorMapper.Transform(r.Context(), err)
				write.ErrorResponse(r.Context(), w, e)
			}
		})
	}
}

func NewServeMux(baseURL string, opts ...ServeMuxOption) *ServeMux {
	m := &ServeMux{
		httpServeMux: http.ServeMux{},
		BaseURL:      baseURL,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.httpServeMux.ServeHTTP(w, r)
}

func (m *ServeMux) HandleFunc(
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) {
	// Split pattern into method and path
	// Pattern format: "METHOD /path" or "/path"
	var p string
	if method, path, found := strings.Cut(pattern, " "); found {
		// Remove base URL from path using TrimPrefix to prevent bypass with duplicate base URLs
		trimmedPath := strings.TrimPrefix(path, m.BaseURL)
		p = method + " " + trimmedPath
	} else {
		// No method prefix, just path
		p = strings.TrimPrefix(pattern, m.BaseURL)
	}

	_, restricted := authz.RestrictionsByAPI[p]
	_, allowed := authz.AllowListByAPI[p]

	if !restricted && !allowed {
		panic("pattern not registered in restrictions or allow list: " + p)
	}

	m.httpServeMux.HandleFunc(pattern, handler)
}

// Handler returns the handler and registered pattern that matches the request.
func (m *ServeMux) Handler(r *http.Request) (http.Handler, string) {
	return m.httpServeMux.Handler(r)
}
