package commands

import (
	"context"
	"errors"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
)

var ErrAuthzLoader = errors.New("error creating authz repo loader")

type contextKey string

const (
	TenantManagerFactoryKey contextKey = "tenantManagerFactory"
)

type CommandFactory struct {
	dbCon *multitenancy.DB
	r     repo.Repo
	gm    *manager.GroupManager
	tm    *manager.TenantManager
}

//nolint:funlen
func NewCommandFactory(
	ctx context.Context,
	cfg *config.Config,
	dbCon *multitenancy.DB,
	svcRegistry serviceapi.Registry,
) (*CommandFactory, error) {
	r := sql.NewRepository(dbCon)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(ctx, r, cfg)
	if authzRepoLoader.AuthzHandler == nil {
		return nil, ErrAuthzLoader
	}

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	cmkAuditor := auditor.New(ctx, cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, err
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, authzRepo)
	if err != nil {
		return nil, err
	}

	cm := manager.NewCertificateManager(ctx, authzRepo, svcRegistry, cfg)
	um := manager.NewUserManager(authzRepo, cmkAuditor)
	tagm := manager.NewTagManager(authzRepo)
	kcm := manager.NewKeyConfigManager(authzRepo, cm, um, tagm, cmkAuditor, eventFactory, cfg)

	sys := manager.NewSystemManager(
		ctx,
		authzRepo,
		authzRepoLoader,
		clientsFactory,
		eventFactory,
		svcRegistry,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		authzRepo,
		svcRegistry,
		manager.NewTenantConfigManager(authzRepo, svcRegistry, cfg),
		kcm,
		um,
		cm,
		eventFactory,
		cmkAuditor,
	)

	migrator, err := db.NewMigrator(r, cfg)
	if err != nil {
		return nil, err
	}

	return &CommandFactory{
		dbCon: dbCon,
		r:     authzRepo,
		gm:    manager.NewGroupManager(authzRepo, svcRegistry, um),
		tm:    manager.NewTenantManager(authzRepo, sys, km, um, cmkAuditor, migrator),
	}, nil
}
