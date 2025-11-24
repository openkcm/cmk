package manager

import (
	"context"
	"crypto/rsa"

	"github.com/google/uuid"

	certissuerv1 "github.com/openkcm/plugin-sdk/proto/plugin/certificate_issuer/v1"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	wf "github.com/openkcm/cmk/internal/workflow"
)

var GetPluginAlgorithm = getPluginAlgorithm
var ValidateSchema = validateSchema

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

func (m *SystemManager) EventSelector(
	ctx context.Context,
	r repo.Repo,
	updatedSystem *model.System,
	oldKeyConfigID *uuid.UUID,
	keyConfig *model.KeyConfiguration,
) (SystemEvent, error) {
	return m.eventSelector(ctx, r, updatedSystem, oldKeyConfigID, keyConfig)
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
