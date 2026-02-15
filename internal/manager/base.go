package manager

import (
	"context"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo"
)

type Manager struct {
	Keys          *KeyManager
	KeyVersions   *KeyVersionManager
	TenantConfigs *TenantConfigManager
	System        System
	KeyConfig     KeyConfigurationAPI
	Tags          Tags
	Labels        Label
	Workflow      Workflow
	Certificates  *CertificateManager
	Group         *GroupManager
	User          User

	Tenant Tenant

	Catalog    *cmkplugincatalog.Registry
	Reconciler *eventprocessor.CryptoReconciler
	Auditor    *auditor.Auditor
}

func New(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
	clientsFactory clients.Factory,
	svcRegistry *cmkplugincatalog.Registry,
	reconciler *eventprocessor.CryptoReconciler,
	asyncClient async.Client,
	migrator db.Migrator,
) *Manager {
	cmkAuditor := auditor.New(ctx, config)
	tenantConfigManager := NewTenantConfigManager(repo, svcRegistry, config)
	userManager := NewUserManager(repo, cmkAuditor)
	certManager := NewCertificateManager(ctx, repo, svcRegistry, &config.Certificates)
	tagManager := NewTagManager(repo)
	keyConfigManager := NewKeyConfigManager(repo, certManager, userManager, tagManager, cmkAuditor, config)
	keyManager := NewKeyManager(
		repo,
		svcRegistry,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		reconciler,
		cmkAuditor,
	)
	systemManager := NewSystemManager(
		ctx,
		repo,
		clientsFactory,
		reconciler,
		svcRegistry,
		config,
		keyConfigManager,
		userManager,
	)
	groupManager := NewGroupManager(repo, svcRegistry, userManager)

	return &Manager{
		Keys:          keyManager,
		KeyVersions:   NewKeyVersionManager(repo, svcRegistry, tenantConfigManager, certManager, cmkAuditor),
		TenantConfigs: tenantConfigManager,
		System:        systemManager,
		KeyConfig:     keyConfigManager,
		Tags:          NewTagManager(repo),
		Labels:        NewLabelManager(repo),
		Workflow: NewWorkflowManager(repo, keyManager, keyConfigManager, systemManager, groupManager, userManager,
			asyncClient, tenantConfigManager, config),
		Certificates: certManager,
		Group:        groupManager,
		User:         userManager,

		Tenant: NewTenantManager(repo, systemManager, keyManager, userManager, cmkAuditor, migrator),

		Catalog:    svcRegistry,
		Reconciler: reconciler,
	}
}
