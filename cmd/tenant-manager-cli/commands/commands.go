package commands

import (
	"context"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
)

type CommandFactory struct {
	dbCon *multitenancy.DB
	r     repo.Repo
	gm    *manager.GroupManager
	tm    *manager.TenantManager
}

func NewCommandFactory(
	ctx context.Context,
	cfg *config.Config,
	dbCon *multitenancy.DB,
	svcRegistry *cmkpluginregistry.Registry,
) (*CommandFactory, error) {
	r := sql.NewRepository(dbCon)

	cmkAuditor := auditor.New(ctx, cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, err
	}

	eventFactory, err := eventprocessor.NewEventFactory(ctx, cfg, r)
	if err != nil {
		return nil, err
	}

	cm := manager.NewCertificateManager(ctx, r, svcRegistry, &cfg.Certificates)
	um := manager.NewUserManager(r, cmkAuditor)
	tagm := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagm, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		clientsFactory,
		eventFactory,
		svcRegistry,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		r,
		svcRegistry,
		manager.NewTenantConfigManager(r, svcRegistry, cfg),
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
		r:     r,
		gm:    manager.NewGroupManager(r, svcRegistry, um),
		tm:    manager.NewTenantManager(r, sys, km, um, cmkAuditor, migrator),
	}, nil
}
