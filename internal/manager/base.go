package manager

import (
	"context"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/featureflags"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/repo"
)

type Manager struct {
	Keys          *KeyManager
	KeyVersions   *KeyVersionManager
	TenantConfigs *TenantConfigManager
	System        System
	SystemGroup   SystemGroup
	KeyConfig     KeyConfigurationAPI
	Tags          Tags
	Labels        Label
	Workflow      Workflow
	Certificates  *CertificateManager
	Group         *GroupManager
	User          User

	Tenant Tenant

	Catalog      serviceapi.Registry
	EventFactory *eventprocessor.EventFactory
	Auditor      *auditor.Auditor
}

//nolint:funlen
func New(
	ctx context.Context,
	r repo.Repo,
	authzLoader *authz_loader.AuthzLoader[
		authz.RepoResourceType, authz.RepoAction],
	config *config.Config,
	clientsFactory clients.Factory,
	svcRegistry serviceapi.Registry,
	eventFactory *eventprocessor.EventFactory,
	asyncClient async.Client,
	migrator db.Migrator,
	flags featureflags.Client,
) *Manager {
	cmkAuditor := auditor.New(ctx, config)
	certManager := NewCertificateManager(ctx, r, svcRegistry, config)
	flagManager := NewFeatureFlags(config, svcRegistry, flags)
	systemGroup := NewSystemGroup(r)
	tenantConfigManager := NewTenantConfigManager(r, svcRegistry, config, certManager, flagManager, systemGroup)
	userManager := NewUserManager(r, cmkAuditor)
	tagManager := NewTagManager(r)
	keyConfigManager := NewKeyConfigManager(r, certManager, userManager,
		tagManager, cmkAuditor, eventFactory, config, tenantConfigManager)
	keyManager := NewKeyManager(
		r,
		svcRegistry,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		eventFactory,
		cmkAuditor,
		asyncClient,
		config,
		flagManager,
	)
	systemManager := NewSystemManager(
		ctx,
		r,
		authzLoader,
		clientsFactory,
		eventFactory,
		svcRegistry,
		config,
		keyConfigManager,
		userManager,
		tenantConfigManager,
	)
	groupManager := NewGroupManager(r, svcRegistry, userManager)

	return &Manager{
		Keys:          keyManager,
		KeyVersions:   NewKeyVersionManager(r, svcRegistry, tenantConfigManager, certManager, cmkAuditor),
		TenantConfigs: tenantConfigManager,
		System:        systemManager,
		SystemGroup:   systemGroup,
		KeyConfig:     keyConfigManager,
		Tags:          NewTagManager(r),
		Labels:        NewLabelManager(r),
		Workflow: NewWorkflowManager(
			r,
			svcRegistry,
			keyManager,
			keyConfigManager,
			systemManager,
			groupManager,
			userManager,
			asyncClient,
			tenantConfigManager,
			config,
		),
		Certificates: certManager,
		Group:        groupManager,
		User:         userManager,

		Tenant: NewTenantManager(r, systemManager, keyManager, userManager, cmkAuditor, migrator),

		Catalog:      svcRegistry,
		EventFactory: eventFactory,
	}
}
