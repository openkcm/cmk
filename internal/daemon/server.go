package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/handlers"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/middleware"
	"github.com/openkcm/cmk/internal/repo/sql"
)

const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 10 * time.Second
	WriteTimeout      = 10 * time.Second
	IdleTimeout       = 120 * time.Second
	ServerLogDomain   = "server daemon"

	APIVersionedNamespace = "/cmk/v1"
	TenantPathParamName   = "tenant"
)

type CmkServer struct {
	cfg              *config.Config
	controller       *cmk.APIController
	clientsFactory   *clients.Factory
	server           *http.Server
	signingKeyLoader *loader.Loader
}

type Server interface {
	Start(ctx context.Context) error
	Close() error
}

func NewCMKServer(
	ctx context.Context,
	cfg *config.Config,
) (*CmkServer, error) {
	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		log.Error(ctx, "error connecting to registry service gRPC server", err)
	}

	dbCon, err := db.StartDB(ctx, cfg.Database, cfg.Provisioning, cfg.DatabaseReplicas)
	if err != nil {
		return nil, oops.In(ServerLogDomain).Wrapf(err, "starting db")
	}

	repo := sql.NewRepository(dbCon)
	controller := cmk.NewAPIController(ctx, repo, *cfg, clientsFactory)

	memoryStorage := keyvalue.NewMemoryStorage[string, []byte]()

	signingKeyLoader, err := loader.Create(
		loader.OnPath(cfg.ClientData.SigningKeysPath),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	if err != nil {
		return nil, oops.In(ServerLogDomain).Wrapf(err, "creating data loader")
	}

	httpServer, err := createHTTPServer(cfg, controller, memoryStorage)
	if err != nil {
		return nil, oops.In(ServerLogDomain).Wrapf(err, "creating http server")
	}

	return &CmkServer{
		cfg:              cfg,
		clientsFactory:   clientsFactory,
		controller:       controller,
		server:           httpServer,
		signingKeyLoader: signingKeyLoader,
	}, nil
}

func (s *CmkServer) Close(ctx context.Context) error {
	err := s.signingKeyLoader.Close()
	if err != nil {
		log.Error(ctx, "failed to stop signing keys loader", err)
	}

	err = s.controller.Manager.Catalog.Close()
	if err != nil {
		return oops.In(ServerLogDomain).Wrapf(err, "closing cmk catalog")
	}

	if s.clientsFactory != nil {
		err = s.clientsFactory.Close()
		if err != nil {
			return oops.In(ServerLogDomain).Wrapf(err, "closing gRPC connection")
		}
	}

	shutdownCtx, shutdownRelease := context.WithTimeout(ctx, s.cfg.HTTP.ShutdownTimeout)
	defer shutdownRelease()

	err = s.server.Shutdown(shutdownCtx)
	if err != nil {
		return oops.In("HTTP Server").
			WithContext(ctx).
			Wrapf(err, "Failed shutting down HTTP server")
	}

	log.Info(ctx, "Completed graceful shutdown of HTTP server")

	return nil
}

func (s *CmkServer) Start(ctx context.Context) error {
	err := s.signingKeyLoader.Start()
	if err != nil {
		log.Error(ctx, "failed to start signing keys loader", err)
	}

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(ctx, "server encountered an error", err)

			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}
	}()

	return nil
}

// setupSwagger loads the swagger file
func setupSwagger() (*openapi3.T, error) {
	swagger, err := cmkapi.GetSwagger()
	if err != nil {
		return nil, errs.Wrapf(err, "failed to load swagger file")
	}
	// Instead of setting Servers list to nil, we only remove the host from the URL.
	// This is because gorilla/mux used by the OAPI validator only allows hosts
	// without periods '.' in the URL. However, we still want to keep
	// the rest of the Server URL to allow matching path prefix with parameterised tenants.
	for _, srv := range swagger.Servers {
		srv.URL = strings.Replace(srv.URL, "{host}", "", 1)
	}

	return swagger, nil
}

func createHTTPServer(
	cfg *config.Config,
	ctr *cmk.APIController,
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
) (*http.Server, error) {
	swagger, err := setupSwagger()
	if err != nil {
		return nil, oops.In(ServerLogDomain).Wrapf(err, "setup swagger")
	}

	// Middlewares run in a FILO. Last middleware on the slice is the first one ran
	// First middleware to run should be the InjectRequestID
	httpHandler := cmkapi.HandlerWithOptions(
		cmkapi.NewStrictHandlerWithOptions(
			ctr,
			[]cmkapi.StrictMiddlewareFunc{},
			cmkapi.StrictHTTPServerOptions{
				RequestErrorHandlerFunc:  handlers.RequestErrorHandlerFunc(),
				ResponseErrorHandlerFunc: handlers.ResponseErrorHandlerFunc(),
			},
		),
		cmkapi.StdHTTPServerOptions{
			BaseURL:          fmt.Sprintf("%s/{%s}", APIVersionedNamespace, TenantPathParamName),
			BaseRouter:       http.NewServeMux(),
			ErrorHandlerFunc: handlers.ParamsErrorHandler(),
			Middlewares: []cmkapi.MiddlewareFunc{
				middleware.ClientDataMiddleware(&cfg.FeatureGates, signingKeyStorage),
				middleware.OAPIMiddleware(swagger),
				middleware.LoggingMiddleware(),
				middleware.PanicRecoveryMiddleware(),
				middleware.InjectMultiTenancy(),
				middleware.InjectRequestID(),
			},
		},
	)

	return &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           httpHandler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		ReadTimeout:       ReadTimeout,
		WriteTimeout:      WriteTimeout,
		IdleTimeout:       IdleTimeout,
	}, nil
}
