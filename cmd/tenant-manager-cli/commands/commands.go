package commands

import (
	"context"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	cmkplugincatalog "github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
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
	ctlg *cmkplugincatalog.Registry,
) (*CommandFactory, error) {
	r := sql.NewRepository(dbCon)

	cmkAuditor := auditor.New(ctx, cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	if err != nil {
		return nil, err
	}

	reconciler, err := eventprocessor.NewCryptoReconciler(
		ctx, cfg, r,
		ctlg, clientsFactory,
	)
	if err != nil {
		return nil, err
	}

	cm := manager.NewCertificateManager(ctx, r, ctlg, &cfg.Certificates)
	um := manager.NewUserManager(r, cmkAuditor)
	tagm := manager.NewTagManager(r)
	kcm := manager.NewKeyConfigManager(r, cm, um, tagm, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		clientsFactory,
		reconciler,
		ctlg,
		cfg,
		kcm,
		um,
	)

	km := manager.NewKeyManager(
		r,
		ctlg,
		manager.NewTenantConfigManager(r, ctlg, cfg),
		kcm,
		um,
		cm,
		reconciler,
		cmkAuditor,
	)

	migrator, err := db.NewMigrator(r, cfg)
	if err != nil {
		return nil, err
	}

	return &CommandFactory{
		dbCon: dbCon,
		r:     r,
		gm:    manager.NewGroupManager(r, ctlg, um),
		tm:    manager.NewTenantManager(r, sys, km, um, cmkAuditor, migrator),
	}, nil
}
