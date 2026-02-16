package manager

import (
	"context"
	"crypto/rsa"

	certissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"

	"github.com/openkcm/cmk/internal/async"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	wf "github.com/openkcm/cmk/internal/workflow"
)

var (
	GetPluginAlgorithm = getPluginAlgorithm
	ValidateSchema     = validateSchema
)

func (m *TenantConfigManager) GetTenantConfigsHyokKeystore() HYOKKeystore {
	return m.getTenantConfigsHyokKeystore()
}

func (m *TenantConfigManager) SetDefaultKeystore(ctx context.Context, keystore *model.KeystoreConfig) error {
	return m.setDefaultKeystore(ctx, keystore)
}

func (si *SystemInformation) SetClient(systemInformation systeminformationv1.SystemInformationServiceClient) {
	si.sisClient = systemInformation
}

func (m *SystemManager) SelectEvent(
	ctx context.Context,
	system *model.System,
	newKeyConfig *model.KeyConfiguration,
) (eventprocessor.Event, error) {
	return m.selectEvent(ctx, system, newKeyConfig)
}

func (m *CertificateManager) SetClient(client certissuerv1.CertificateIssuerServiceClient) {
	m.certIssuerClient = client
}

func (m *CertificateManager) SetRotationThreshold(days int) {
	m.cfg.RotationThresholdDays = days
}

func (m *CertificateManager) SetPrivateKeyGenerator(generator func() (*rsa.PrivateKey, error)) {
	m.privateKeyGenerator = generator
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

func (m *CertificateManager) GetCertificateByPurpose(
	ctx context.Context,
	purpose model.CertificatePurpose,
) (*model.Certificate, bool, error) {
	return m.getCertificateByPurpose(ctx, purpose)
}

func (w *WorkflowManager) CreateWorkflowTransitionNotificationTask(
	ctx context.Context,
	workflow model.Workflow,
	transition wf.Transition,
	recipients []string,
) error {
	return w.createWorkflowTransitionNotificationTask(ctx, workflow, transition, recipients)
}

func (w *WorkflowManager) SetAsyncClient(client async.Client) {
	w.asyncClient = client
}
