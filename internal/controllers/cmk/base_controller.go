package cmk

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
)

// APIController handles API requests related to CMK (Customer Managed Keys).
type APIController struct {
	pluginCatalog *plugincatalog.Catalog
	Repository    repo.Repo
	Manager       *manager.Manager
	config        *config.Config
}

// NewAPIController creates a new instance of APIController with the provided Repository.
// It initializes a logger for the controller.
func NewAPIController(
	ctx context.Context,
	r repo.Repo,
	config config.Config,
	clientsFactory *clients.Factory,
) *APIController {
	ctlg, err := catalog.New(ctx, config)
	if err != nil {
		log.Error(ctx, "Failed to load plugin", err)
	}

	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, &config, r, ctlg)
	if err != nil {
		log.Error(ctx, "Failed to create event reconciler", err)
	} else {
		err = reconciler.Start(ctx)
		if err != nil {
			log.Error(ctx, "Failed to start event reconciler", err)
		}
	}

	var asyncClient async.Client

	asyncApp, err := async.New(&config)
	if err != nil {
		log.Error(ctx, "Failed to create async app", err)
	} else {
		asyncClient = asyncApp.Client()
	}

	return &APIController{
		Manager:       manager.New(ctx, r, &config, clientsFactory, ctlg, reconciler, asyncClient),
		config:        &config,
		pluginCatalog: ctlg,
	}
}
