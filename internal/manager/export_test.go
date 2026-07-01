package manager

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/openkcm/cmk/internal/async"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
	"github.com/openkcm/cmk/internal/repo"
	wf "github.com/openkcm/cmk/internal/workflow"
)

var GetPluginAlgorithm = getPluginAlgorithm

func (m *TenantConfigManager) GetTenantConfigsHyokKeystore() HYOKKeystore {
	return m.getTenantConfigsHyokKeystore()
}

func (m *SystemInformation) SetClient(systemInformation systeminformation.SystemInformation) {
	m.svc = systemInformation
}

func (m *SystemManager) SelectEvent(
	ctx context.Context,
	system *model.System,
	newKeyConfig *model.KeyConfiguration,
) (eventprocessor.Event, error) {
	return m.selectEvent(ctx, system, newKeyConfig)
}

func (m *CertificateManager) SetCertIssuerService(certIssuer certificateissuer.CertificateIssuer) {
	m.certIssuer = certIssuer
}

func (m *CertificateManager) SetRotationThreshold(days int) {
	m.cfg.Certificates.RotationThresholdDays = days
}

func (m *CertificateManager) SetPrivateKeyGenerator(generator func() (*rsa.PrivateKey, error)) {
	m.privateKeyGenerator = generator
}

func (m *CertificateManager) GetDefaultKeystoreClientCert(
	ctx context.Context,
	localityID string,
	commonName string,
	purpose model.CertificatePurpose,
) (*model.Certificate, error) {
	return m.getDefaultKeystoreClientCert(ctx, localityID, commonName, purpose)
}

func (m *CertificateManager) GetDefaultHYOKClientCert(
	ctx context.Context,
) (*model.Certificate, error) {
	return m.getDefaultHYOKClientCert(ctx)
}

func (m *CertificateManager) ResolveRotationLocality(certPEM string) []string {
	return m.resolveRotationLocality(certPEM)
}

func (m *CertificateManager) SetRegion(region string) {
	m.cfg.Landscape.Region = region
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

func (w *WorkflowManager) ValidateApproverCount(
	ctx context.Context,
	workflow *model.Workflow,
	minimumApprovals int,
) (bool, error) {
	return w.validateApproverCount(ctx, workflow, minimumApprovals)
}

func (m *TenantManager) UnmapSystemErrorCanContinue(ctx context.Context, err error) OffboardingStatus {
	return m.unmapSystemErrorCanContinue(ctx, err)
}

func (m *TenantManager) SetSystemForTests(sys System) {
	m.sys = sys
}

func (m *TenantManager) SetRepoForTests(repo repo.Repo) {
	m.repo = repo
}

func (w *WorkflowManager) GetApproverGroupsFromLegacyField(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.Group, error) {
	return w.getApproverGroupsFromLegacyField(ctx, workflow)
}

func (km *KeyManager) ExportedHandleNewKeyVersion(
	ctx context.Context,
	key *model.Key,
	keyResp *keymanagement.GetKeyResponse,
	rotationTime *time.Time,
) error {
	return km.handleNewKeyVersion(ctx, key, keyResp, rotationTime)
}
