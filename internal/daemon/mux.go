package daemon

import (
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/openkcm/cmk/internal/api/cmk/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/authz"
)

// PatternChecker is a function that reports whether a URL pattern
// is allowed to be registered on the mux.
type PatternChecker func(pattern string) bool

func defaultPatternChecker(p string) bool {
	_, restricted := authz.RestrictionsByAPI[p]
	_, allowed := authz.AllowListByAPI[p]
	return restricted || allowed
}

type ServeMux struct {
	httpServeMux   http.ServeMux
	BaseURL        string
	patternChecker PatternChecker
}

type ServeMuxOption func(*ServeMux)

// WithPatternChecker replaces the default authz-based pattern checker with the
// provided function. The function receives the stripped pattern ("METHOD /path")
// and should return true if the pattern is allowed.
func WithPatternChecker(checker PatternChecker) ServeMuxOption {
	return func(m *ServeMux) {
		m.patternChecker = checker
	}
}

// WithSwaggerUI adds an endpoint to serve Swagger UI at /cmk/v1/swagger
// The Swagger UI will load the OpenAPI spec inline without requiring a separate endpoint
// Note: The {tenant} parameter is removed from the base URL for this endpoint
func WithSwaggerUI(swagger *openapi3.T) ServeMuxOption {
	return func(m *ServeMux) {
		// Remove {tenant} parameter from base URL for swagger endpoint
		swaggerBaseURL := strings.TrimSuffix(m.BaseURL, "/{tenant}")
		pattern := "GET " + swaggerBaseURL + "/swagger"
		m.httpServeMux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			html, err := SwaggerUI(swagger)
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
		httpServeMux:   http.ServeMux{},
		BaseURL:        baseURL,
		patternChecker: defaultPatternChecker,
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

	if !m.patternChecker(p) {
		panic("pattern not registered in restrictions or allow list: " + p)
	}

	m.httpServeMux.HandleFunc(pattern, handler)
}

// Handler returns the handler and registered pattern that matches the request.
func (m *ServeMux) Handler(r *http.Request) (http.Handler, string) {
	return m.httpServeMux.Handler(r)
}
