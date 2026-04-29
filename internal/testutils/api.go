package testutils

import (
	"bytes"
	"context"
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
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	md "github.com/oapi-codegen/nethttp-middleware"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/handlers"
	"github.com/openkcm/cmk/internal/middleware"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo/sql"
)

const TestCertURL = "https://aia.pki.co.test.com/aia/TEST%20Cloud%20Root%20CA.crt"

const TestHostPrefix = "https://kms.test/cmk/v1/"

type TestAPIServerConfig struct {
	Plugins            []catalog.BuiltInPlugin       // Plugins only set if needed
	GRPCCon            *commongrpc.DynamicClientConn // GRPCClient only set if needed
	Config             config.Config
	EnableClientDataMW bool                   // Enable ClientDataMiddleware (default: false for backward compatibility)
	SigningKeyStorage  keyvalue.ReadOnlyStringToBytesStorage // Optional: provide custom signing key storage
}

// NewAPIServer creates a new API server with the given database connection
func NewAPIServer(
	tb testing.TB,
	dbCon *multitenancy.DB,
	testCfg TestAPIServerConfig,
) cmkapi.ServeMux {
	tb.Helper()

	cfg := testCfg.Config

	ps, psCfg := NewTestPlugins(testCfg.Plugins...)
	cfg.Plugins = psCfg

	cfg.Certificates.RootCertURL = TestCertURL
	if cfg.Database == (config.Database{}) {
		cfg.Database = TestDB
	}

	r := sql.NewRepository(dbCon)

	var (
		factory clients.Factory
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

	migrator, err := db.NewMigrator(r, &cfg)
	assert.NoError(tb, err)

	ctx := tb.Context()
	authzAPILoader := authz_loader.NewAPIAuthzLoader(ctx, r, &cfg)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, r, &cfg)

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	svcRegistry, err := cmkpluginregistry.New(tb.Context(), &cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(tb, err)

	controller := cmk.NewAPIController(tb.Context(), authzRepo, &cfg, factory, migrator, svcRegistry, authzAPILoader)

	return startAPIServer(tb, controller, testCfg)
}

//nolint:funlen
func startAPIServer(
	tb testing.TB,
	controller *cmk.APIController,
	testCfg TestAPIServerConfig,
) cmkapi.ServeMux {
	tb.Helper()

	strictController := cmkapi.NewStrictHandlerWithOptions(
		controller,
		[]cmkapi.StrictMiddlewareFunc{},
		cmkapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  handlers.RequestErrorHandlerFunc(),
			ResponseErrorHandlerFunc: handlers.ResponseErrorHandlerFunc(),
		},
	)

	r := daemon.NewServeMux(constants.BasePath)

	openapi3filter.RegisterBodyDecoder(
		"application/merge-patch+json",
		openapi3filter.JSONBodyDecoder,
	)

	swagger, _ := daemon.SetupSwagger()

	mws := []cmkapi.MiddlewareFunc{
		md.OapiRequestValidatorWithOptions(swagger, &md.Options{
			ErrorHandlerWithOpts:  handlers.OAPIValidatorHandler,
			SilenceServersWarning: true,
			Options: openapi3filter.Options{
				AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
				IncludeResponseStatus: true,
			},
		}),
	}

	// Middlewares are applied from last to first.
	// Keep Authz before ClientData in the slice so ClientData runs first at request time.
	mws = append(mws,
		middleware.AuthzMiddleware(controller),
		middleware.LoggingMiddleware(),
		middleware.PanicRecoveryMiddleware(),
		middleware.InjectMultiTenancy(),
		middleware.InjectRequestID(),
	)

	// Add ClientDataMiddleware if enabled.
	// It must be appended after Authz in the slice so it runs before Authz.
	if testCfg.EnableClientDataMW {
		signingKeyStorage := testCfg.SigningKeyStorage
		if signingKeyStorage == nil {
			// Create default test signing key storage if not provided
			signingKeyStorage = NewTestSigningKeyStorage(tb)
		}

		// Default auth context fields for testing
		authContextFields := []string{"client_id", "issuer", "multitenancy_ref"}

		// Use test role getter
		roleGetter := NewTestRoleGetter()

		mws = append(mws, middleware.ClientDataMiddleware(
			signingKeyStorage,
			authContextFields,
			roleGetter,
		))
	}

	cmkapi.HandlerWithOptions(strictController,
		cmkapi.StdHTTPServerOptions{
			BaseRouter:       r,
			BaseURL:          constants.BasePath,
			ErrorHandlerFunc: handlers.ParamsErrorHandler(),
			Middlewares:      mws,
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
	Method            string // HTTP Method
	Endpoint          string
	Tenant            string    // TenantID
	Body              io.Reader // Only need to be set for POST/PATCH. Used with the WithString and WithJSON
	Headers           http.Header
	AdditionalContext map[any]any // Deprecated: Use Headers with signed client data instead
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
//
//nolint:cyclop
func NewHTTPRequest(tb testing.TB, opt RequestOptions) *http.Request {
	tb.Helper()

	ctx := tb.Context()

	// Legacy support: inject AdditionalContext if provided and ClientDataMiddleware is not enabled
	// When ClientDataMiddleware is enabled, AdditionalContext is ignored in favor of Headers
	if len(opt.AdditionalContext) > 0 && opt.Headers == nil {
		//nolint: fatcontext
		for k, v := range opt.AdditionalContext {
			ctx = context.WithValue(ctx, k, v)
		}
	}

	r, err := http.NewRequestWithContext(
		ctx,
		opt.Method,
		GetTestURL(tb, opt.Tenant, opt.Endpoint),
		opt.Body,
	)
	assert.NoError(tb, err)

	switch opt.Method {
	case http.MethodGet, http.MethodDelete:
	case http.MethodHead, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		// We do not actually support these but never-the-less we might want
		// to test against them
	case http.MethodPost, http.MethodPut:
		r.Header.Set("Content-Type", "application/json")
	case http.MethodPatch:
		r.Header.Set("Content-Type", "application/merge-patch+json")
	default:
		assert.Fail(tb, "HTTP Method not supported!")
	}

	// Apply provided headers
	if opt.Headers != nil {
		for key, values := range opt.Headers {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
	}

	return r
}

// MakeHTTPRequest creates an HTTP method and gets its response for it
// On POST/PATCH methods, RequestOptions body should use WithString/WithJSON methods
func MakeHTTPRequest(tb testing.TB, server cmkapi.ServeMux, opt RequestOptions) *httptest.ResponseRecorder {
	tb.Helper()

	req := NewHTTPRequest(tb, opt)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	return w
}
