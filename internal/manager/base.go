package manager

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/repo"
)

type Manager struct {
	Keys                 *KeyManager
	KeyVersions          *KeyVersionManager
	TenantConfigs        *TenantConfigManager
	System               System
	KeyConfig            KeyConfigurationAPI
	KeyConfigTags        KeyConfigurationTag
	KeyConfigurationTags KeyConfigurationTag
	Labels               Label
	Workflow             Workflow
	Certificates         *CertificateManager
	Group                *GroupManager

	Tenant Tenant

	Catalog    *plugincatalog.Catalog
	Reconciler *eventprocessor.CryptoReconciler
	Auditor    *auditor.Auditor
}

func New(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
	clientsFactory clients.Factory,
	catalog *plugincatalog.Catalog,
	reconciler *eventprocessor.CryptoReconciler,
	asyncClient async.Client,
) *Manager {
	cmkAuditor := auditor.New(ctx, config)
	tenantConfigManager := NewTenantConfigManager(repo, catalog)
	certManager := NewCertificateManager(ctx, repo, catalog, &config.Certificates)
	keyConfigManager := NewKeyConfigManager(repo, certManager, cmkAuditor, config)
	keyManager := NewKeyManager(repo, catalog, tenantConfigManager, keyConfigManager, certManager, reconciler, cmkAuditor)
	systemManager := NewSystemManager(ctx, repo, clientsFactory, reconciler, catalog, config, keyConfigManager)
	groupManager := NewGroupManager(repo, catalog)

	return &Manager{
		Keys:          keyManager,
		KeyVersions:   NewKeyVersionManager(repo, catalog, tenantConfigManager, certManager, cmkAuditor),
		TenantConfigs: tenantConfigManager,
		System:        systemManager,
		KeyConfig:     keyConfigManager,
		KeyConfigTags: NewKeyConfigurationTagManager(repo),
		Labels:        NewLabelManager(repo),
		Workflow: NewWorkflowManager(repo, keyManager, keyConfigManager, systemManager, groupManager,
			asyncClient, tenantConfigManager),
		Certificates: certManager,
		Group:        groupManager,

		Tenant: NewTenantManager(repo, systemManager, keyManager, cmkAuditor),

		Catalog:    catalog,
		Reconciler: reconciler,
	}
}
