package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/async"
	authzmodel "github.com/openkcm/cmk/internal/authz-model"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
)

// APIController handles API requests related to CMK (Customer Managed Keys).
type APIController struct {
	pluginCatalog *cmkpluginregistry.Registry
	Repository    repo.Repo
	Manager       *manager.Manager
	config        *config.Config
	AuthzEngine   *authzmodel.Engine
}

// NewAPIController creates a new instance of APIController with the provided Repository.
// It initializes a logger for the controller.
func NewAPIController(
	ctx context.Context,
	r repo.Repo,
	config *config.Config,
	clientsFactory clients.Factory,
	migrator db.Migrator,
) *APIController {
	svcRegistry, err := cmkpluginregistry.New(ctx, config)
	if err != nil {
		log.Error(ctx, "Failed to load plugin", err)
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, config, r)
	if err != nil {
		log.Error(ctx, "Failed to create event factory", err)
	}

	var asyncClient async.Client

	asyncApp, err := async.New(config)
	if err != nil {
		log.Error(ctx, "Failed to create async app", err)
	} else {
		asyncClient = asyncApp.Client()
	}

	return &APIController{
		Manager:       manager.New(ctx, r, config, clientsFactory, svcRegistry, eventFactory, asyncClient, migrator),
		config:        config,
		pluginCatalog: svcRegistry,
		AuthzEngine:   authzmodel.NewEngine(ctx, r, config),
	}
}
