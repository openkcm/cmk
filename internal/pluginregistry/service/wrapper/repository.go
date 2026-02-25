package servicewrapper

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
)

const (
	certificateIssuerType        = "CertificateIssuer"
	certificateIssuerServiceType = "CertificateIssuerService"

	notificationType        = "Notification"
	notificationServiceType = "NotificationService"

	systemInformationType        = "SystemInformation"
	systemInformationServiceType = "SystemInformationService"

	identityManagementType        = "IdentityManagement"
	identityManagementServiceType = "IdentityManagementService"

	keystoreManagementType = "KeystoreProvider"
	keyManagementType      = "KeystoreInstanceKeyOperation"
)

type Repository struct {
	identityManagementRepository
	certificateIssuerRepository
	notificationRepository
	systemInformationRepository
	keystoreManagementRepository
	keyManagementRepository

	RawCatalog *catalog.Catalog
}

func (repo *Repository) Plugins() map[string]api.PluginRepo {
	return map[string]api.PluginRepo{
		identityManagementType:        &repo.identityManagementRepository,
		identityManagementServiceType: &repo.identityManagementRepository,
		certificateIssuerType:         &repo.certificateIssuerRepository,
		certificateIssuerServiceType:  &repo.certificateIssuerRepository,
		notificationType:              &repo.notificationRepository,
		notificationServiceType:       &repo.notificationRepository,
		systemInformationType:         &repo.systemInformationRepository,
		systemInformationServiceType:  &repo.systemInformationRepository,
		keystoreManagementType:        &repo.keystoreManagementRepository,
		keyManagementType:             &repo.keyManagementRepository,
	}
}

func (repo *Repository) Services() []api.ServiceRepo {
	return nil
}

func (repo *Repository) Reconfigure(ctx context.Context) {
	repo.RawCatalog.Reconfigure(ctx)
}

func (repo *Repository) Close() error {
	if repo.RawCatalog == nil {
		return nil
	}

	return repo.RawCatalog.Close()
}

func CreateServiceRepository(
	ctx context.Context,
	config catalog.Config,
	builtIns ...catalog.BuiltInPlugin,
) (*Repository, error) {
	repo := &Repository{}

	var err error
	repo.RawCatalog, err = catalog.New(ctx, config, repo, builtIns...)
	if err != nil {
		return nil, err
	}

	return repo, nil
}
