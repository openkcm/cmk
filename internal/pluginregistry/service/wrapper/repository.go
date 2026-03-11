package servicewrapper

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
)

const (
	CertificateIssuerType        = "CertificateIssuer"
	CertificateIssuerServiceType = "CertificateIssuerService"

	NotificationType        = "Notification"
	NotificationServiceType = "NotificationService"

	SystemInformationType        = "SystemInformation"
	SystemInformationServiceType = "SystemInformationService"

	IdentityManagementType        = "IdentityManagement"
	IdentityManagementServiceType = "IdentityManagementService"

	KeystoreManagementType = "KeystoreProvider"
	KeyManagementType      = "KeystoreInstanceKeyOperation"
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
		IdentityManagementType:        &repo.identityManagementRepository,
		IdentityManagementServiceType: &repo.identityManagementRepository,
		CertificateIssuerType:         &repo.certificateIssuerRepository,
		CertificateIssuerServiceType:  &repo.certificateIssuerRepository,
		NotificationType:              &repo.notificationRepository,
		NotificationServiceType:       &repo.notificationRepository,
		SystemInformationType:         &repo.systemInformationRepository,
		SystemInformationServiceType:  &repo.systemInformationRepository,
		KeystoreManagementType:        &repo.keystoreManagementRepository,
		KeyManagementType:             &repo.keyManagementRepository,
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
