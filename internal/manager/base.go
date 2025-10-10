package manager

import (
	"context"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/repo"
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
	Group                Group
	Authz                *AuthzManager

	Tenant Tenant

	Catalog    *plugincatalog.Catalog
	Reconciler *eventprocessor.CryptoReconciler
	Auditor    *auditor.Auditor
}

func New(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
	clientsFactory *clients.Factory,
	catalog *plugincatalog.Catalog,
	reconciler *eventprocessor.CryptoReconciler,
) *Manager {
	cmkAuditor := auditor.New(ctx, config)
	tenantConfigManager := NewTenantConfigManager(repo, catalog)
	certManager := NewCertificateManager(ctx, repo, catalog, &config.Certificates)
	keyConfigManager := NewKeyConfigManager(repo, certManager, config)
	keyManager := NewKeyManager(repo, catalog, tenantConfigManager, keyConfigManager, certManager, reconciler, cmkAuditor)
	systemManager := NewSystemManager(ctx, repo, clientsFactory, reconciler, catalog, cmkAuditor, config)
	authzManager := NewAuthzManager(ctx, repo)

	return &Manager{
		Keys:          keyManager,
		KeyVersions:   NewKeyVersionManager(repo, catalog, tenantConfigManager, certManager, cmkAuditor),
		TenantConfigs: tenantConfigManager,
		System:        systemManager,
		KeyConfig:     keyConfigManager,
		KeyConfigTags: NewKeyConfigurationTagManager(repo),
		Labels:        NewLabelManager(repo),
		Workflow:      NewWorkflowManager(repo, keyManager, keyConfigManager, systemManager, &config.Workflows),
		Certificates:  certManager,
		Group:         NewGroupManager(repo),
		Authz:         authzManager,

		Tenant: NewTenantManager(repo),

		Catalog:    catalog,
		Reconciler: reconciler,
	}
}
