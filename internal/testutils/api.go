package testutils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	md "github.com/oapi-codegen/nethttp-middleware"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/handlers"
	"github.com/openkcm/cmk/internal/middleware"
	"github.com/openkcm/cmk/internal/repo/sql"
)

const TestCertURL = "https://aia.pki.co.test.com/aia/TEST%20Cloud%20Root%20CA.crt"

const TestHostPrefix = "https://kms.test/cmk/v1/"

type TestAPIServerConfig struct {
	Plugins []MockPlugin                  // HashiCorp plugins only set if needed
	GRPCCon *commongrpc.DynamicClientConn // GRPCClient only set if needed
	Config  config.Config
}

// NewAPIServer creates a new API server with the given database connection
func NewAPIServer(
	tb testing.TB,
	db *multitenancy.DB,
	testCfg TestAPIServerConfig,
) *http.ServeMux {
	tb.Helper()

	cfg := testCfg.Config

	cfg.Plugins = SetupMockPlugins(testCfg.Plugins...)
	cfg.Certificates.RootCertURL = TestCertURL
	cfg.Database = TestDB

	r := sql.NewRepository(db)

	var (
		factory *clients.Factory
		err     error
	)

	if testCfg.GRPCCon != nil {
		factory, err = clients.NewFactory(config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: testCfg.GRPCCon.Target(),
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		})
		assert.NoError(tb, err)
	} else {
		factory, err = clients.NewFactory(config.Services{})
		assert.NoError(tb, err)
	}

	tb.Cleanup(func() {
		assert.NoError(tb, factory.Close())
	})

	controller := cmk.NewAPIController(tb.Context(), r, cfg, factory)

	return startAPIServer(controller)
}

func startAPIServer(
	controller *cmk.APIController,
) *http.ServeMux {
	strictController := cmkapi.NewStrictHandlerWithOptions(
		controller,
		[]cmkapi.StrictMiddlewareFunc{},
		cmkapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  handlers.RequestErrorHandlerFunc(),
			ResponseErrorHandlerFunc: handlers.ResponseErrorHandlerFunc(),
		},
	)

	r := http.NewServeMux()

	openapi3filter.RegisterBodyDecoder(
		"application/merge-patch+json",
		openapi3filter.JSONBodyDecoder,
	)

	swagger, _ := cmkapi.GetSwagger()
	for _, srv := range swagger.Servers {
		srv.URL = strings.Replace(srv.URL, "{host}", "", 1)
	}

	cmkapi.HandlerWithOptions(strictController,
		cmkapi.StdHTTPServerOptions{
			BaseRouter:       r,
			BaseURL:          "/cmk/v1/{tenant}",
			ErrorHandlerFunc: handlers.ParamsErrorHandler(),
			Middlewares: []cmkapi.MiddlewareFunc{
				md.OapiRequestValidatorWithOptions(swagger, &md.Options{
					ErrorHandlerWithOpts:  handlers.OAPIValidatorHandler,
					SilenceServersWarning: true,
					Options: openapi3filter.Options{
						AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
						IncludeResponseStatus: true,
					},
				}),
				middleware.LoggingMiddleware(),
				middleware.PanicRecoveryMiddleware(),
				middleware.InjectMultiTenancy(),
				middleware.InjectRequestID(),
			},
		})

	return r
}

func GetTestURL(tb testing.TB, tenant, path string) string {
	tb.Helper()

	if tenant == "" {
		tenant = TestTenant
	}

	u, err := url.JoinPath(TestHostPrefix, tenant, path)
	assert.NoError(tb, err)

	uHex, err := url.PathUnescape(u)
	assert.NoError(tb, err)

	return uHex
}

type RequestOptions struct {
	Method   string // HTTP Method
	Endpoint string
	Tenant   string    // TenantID
	Body     io.Reader // Only need to be set for POST/PATCH Methods. Used with the WithString and WithJSON methods
	Headers  map[string]string
}

// WithString is a helper function that converts a string to an io.Reader.
// It is intended to be used as the Body field in RequestOptions when making HTTP requests in tests.
func WithString(tb testing.TB, i any) io.Reader {
	tb.Helper()

	str, ok := i.(string)
	if !ok {
		assert.Fail(tb, "Must provide a string")
	}

	return strings.NewReader(str)
}

// WithJSON is a helper function that marshals an object to JSON and returns an io.Reader.
// It is intended to be used as the Body field in RequestOptions when making HTTP requests in tests.
func WithJSON(tb testing.TB, i any) io.Reader {
	tb.Helper()

	bs, err := json.Marshal(i)
	assert.NoError(tb, err)

	return bytes.NewReader(bs)
}

// GetJSONBody is used to get a response out of an HTTP Body encoded as JSON
// For error responses use cmkapi.ErrorMessage as it's type
func GetJSONBody[t any](tb testing.TB, w *httptest.ResponseRecorder) t {
	tb.Helper()

	var typ t

	err := json.Unmarshal(w.Body.Bytes(), &typ)
	assert.NoError(tb, err)

	return typ
}

// NewHTTPRequest builds an HTTP Request it sets default content-types for certain Methods
func NewHTTPRequest(tb testing.TB, opt RequestOptions) *http.Request {
	tb.Helper()

	r, err := http.NewRequestWithContext(
		tb.Context(),
		opt.Method,
		GetTestURL(tb, opt.Tenant, opt.Endpoint),
		opt.Body,
	)
	assert.NoError(tb, err)

	switch opt.Method {
	case http.MethodGet, http.MethodDelete:
	case http.MethodPost, http.MethodPut:
		r.Header.Set("Content-Type", "application/json")
	case http.MethodPatch:
		r.Header.Set("Content-Type", "application/merge-patch+json")
	default:
		assert.Fail(tb, "HTTP Method not supported!")
	}

	for k, v := range opt.Headers {
		r.Header.Add(k, v)
	}

	return r
}

// MakeHTTPRequest creates an HTTP method and gets its response for it
// On POST/PATCH methods, RequestOptions body should use WithString/WithJSON methods
func MakeHTTPRequest(tb testing.TB, server *http.ServeMux, opt RequestOptions) *httptest.ResponseRecorder {
	tb.Helper()

	req := NewHTTPRequest(tb, opt)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	return w
}
