package commands

import (
	"context"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
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
	ctlg *plugincatalog.Catalog,
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
	kcm := manager.NewKeyConfigManager(r, cm, cmkAuditor, cfg)

	sys := manager.NewSystemManager(
		ctx,
		r,
		clientsFactory,
		reconciler,
		ctlg,
		cfg,
		kcm,
	)

	km := manager.NewKeyManager(
		r,
		ctlg,
		manager.NewTenantConfigManager(r, ctlg),
		kcm,
		cm,
		reconciler,
		cmkAuditor,
	)

	return &CommandFactory{
		dbCon: dbCon,
		r:     r,
		gm:    manager.NewGroupManager(r, ctlg),
		tm:    manager.NewTenantManager(r, sys, km, cmkAuditor),
	}, nil
}
