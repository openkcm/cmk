package manager

import (
	"context"
	"crypto/rsa"

	certissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	notificationv1 "github.com/openkcm/plugin-sdk/proto/plugin/notification/v1"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"

	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/model"
)

var GetPluginAlgorithm = getPluginAlgorithm

func (m *TenantConfigManager) GetTenantConfigsHyokKeystore() HYOKKeystore {
	return m.getTenantConfigsHyokKeystore()
}

func (m *TenantConfigManager) SetDefaultKeystore(ctx context.Context, ksConfig *model.KeystoreConfiguration) error {
	return m.setDefaultKeystore(ctx, ksConfig)
}

func (si *SystemInformation) SetClient(systemInformation systeminformationv1.SystemInformationServiceClient) {
	si.sisClient = systemInformation
}

func (m *SystemManager) GetClient() *systems.Client {
	return m.registry.System()
}

func (m *CertificateManager) SetClient(client certissuerv1.CertificateIssuerServiceClient) {
	m.certIssuerClient = client
}

func (m *CertificateManager) SetPrivateKeyGenerator(generator func() (*rsa.PrivateKey, error)) {
	m.privateKeyGenerator = generator
}

func (m *NotificationManager) SetClient(notificationClient notificationv1.NotificationServiceClient) {
	m.notificationClient = notificationClient
}

func (m *CertificateManager) GetDefaultKeystoreClientCert(
	ctx context.Context,
	localityID string,
	commonName string,
) (*model.Certificate, error) {
	return m.getDefaultKeystoreClientCert(ctx, localityID, commonName)
}

func (m *CertificateManager) GetDefaultHYOKClientCert(
	ctx context.Context,
) (*model.Certificate, error) {
	return m.getDefaultHYOKClientCert(ctx)
}
