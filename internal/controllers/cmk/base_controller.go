package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
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
	AuthzLoader   *authz_loader.AuthzLoader[authz.APIResourceTypeName, authz.APIAction]
}

// NewAPIController creates a new instance of APIController with the provided Repository.
// It initializes a logger for the controller.
func NewAPIController(
	ctx context.Context,
	r repo.Repo,
	config *config.Config,
	clientsFactory clients.Factory,
	migrator db.Migrator,
	svcRegistry *cmkpluginregistry.Registry,
	authzRepoLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName, authz.RepoAction],
	authzAPILoader *authz_loader.AuthzLoader[authz.APIResourceTypeName, authz.APIAction],
) *APIController {
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
		Manager: manager.New(ctx, r, authzRepoLoader, config, clientsFactory,
			svcRegistry, eventFactory, asyncClient, migrator),
		config:        config,
		pluginCatalog: svcRegistry,
		AuthzLoader:   authzAPILoader,
	}
}
