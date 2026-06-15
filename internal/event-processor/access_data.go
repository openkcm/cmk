package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	pluginHelpers "github.com/openkcm/cmk/utils/plugins"
)

var (
	errGetDefaultKeystoreConfig = errors.New("failed to get default keystore config")
	errSetDefaultKeystoreConfig = errors.New("failed to set default keystore config")
	errLoadCryptoCerts          = errors.New("failed to load crypto certificates")
	errGetRoleManagementCert    = errors.New("failed to get role management certificate")
	errGrantTrustFailed         = errors.New("failed to grant trust to certificate")
	errRemoveTrustFailed        = errors.New("failed to remove trust from certificate")
	errPluginCatalogUnavailable = errors.New("no keystore management plugin available")
)

// CryptoAccessDataSyncer builds fresh crypto access data by syncing trust for all
// configured crypto certificates. It uses only repo and svcRegistry.
type CryptoAccessDataSyncer struct {
	cfg         *config.Config
	repo        repo.Repo
	svcRegistry serviceapi.Registry
}

func NewCryptoAccessDataSyncer(
	cfg *config.Config,
	r repo.Repo,
	svcRegistry serviceapi.Registry,
) *CryptoAccessDataSyncer {
	return &CryptoAccessDataSyncer{
		cfg:         cfg,
		repo:        r,
		svcRegistry: svcRegistry,
	}
}

// SyncAndGetCryptoAccessData builds fresh crypto access data for the tenant in ctx.
// Returns empty data if no crypto certificates are configured.
func (s *CryptoAccessDataSyncer) SyncAndGetCryptoAccessData(
	ctx context.Context,
) (map[string]map[string]any, error) {
	cryptoCerts, err := s.getCryptoCertificates(ctx)
	if err != nil {
		return nil, err
	}

	if len(cryptoCerts) == 0 {
		return model.KeyAccessData{}, nil
	}

	log.Info(ctx, "Syncing crypto access data", slog.Int("certCount", len(cryptoCerts)))

	ksConfig, err := s.getDefaultKeystoreConfig(ctx)
	if err != nil {
		return nil, err
	}

	if ksConfig.CryptoAccessData == nil {
		ksConfig.CryptoAccessData = make(map[string]model.CryptoConfig)
	}

	cryptoAccessData := make(model.KeyAccessData)

	keystoreConfigNeedsUpdated := false
	for i := range cryptoCerts {
		updated, err := s.syncCert(ctx, &cryptoCerts[i], ksConfig, cryptoAccessData)
		keystoreConfigNeedsUpdated = keystoreConfigNeedsUpdated || updated
		if err != nil {
			return nil, err
		}
	}

	if keystoreConfigNeedsUpdated {
		err = s.setDefaultKeystoreConfig(ctx, ksConfig)
		if err != nil {
			return nil, err
		}
	}

	log.Info(ctx, "Crypto access data synced", slog.Int("regions", len(cryptoAccessData)))

	return cryptoAccessData, nil
}

//nolint:funlen
func (s *CryptoAccessDataSyncer) syncCert(
	ctx context.Context,
	cert *model.ClientCertificate,
	ksConfig *model.KeystoreConfig,
	cryptoAccessData model.KeyAccessData,
) (bool, error) {
	subject := cert.Subject.FormatSubjectWithSlashSeparatedOUs()
	cryptoCfg, exists := ksConfig.CryptoAccessData[cert.Name]

	subjectChanged := exists && cryptoCfg.Subject != subject
	if exists && !subjectChanged {
		log.Debug(ctx, "Crypto cert access up to date, skipping", slog.String("cert", cert.Name))
		cryptoAccessData[cert.Name] = cryptoCfg.AccessData
		cryptoAccessData[cert.Name][model.CertificateSubjectKey] = subject
		return false, nil
	}

	if subjectChanged {
		err := s.removeCryptoRoleTrust(ctx, cryptoCfg.AccessData, ksConfig)
		if err != nil {
			log.Warn(ctx, "Failed to remove old crypto role trust",
				slog.String("cert", cert.Name),
				slog.String("subject", cryptoCfg.Subject),
				log.ErrorAttr(err),
			)
		} else {
			log.Info(ctx, "Old crypto role trust removed",
				slog.String("cert", cert.Name),
				slog.String("oldSubject", cryptoCfg.Subject),
			)
		}
	}

	if subjectChanged {
		log.Info(ctx, "Crypto cert subject changed, re-granting trust",
			slog.String("cert", cert.Name),
			slog.String("oldSubject", cryptoCfg.Subject),
			slog.String("newSubject", subject),
		)
	} else {
		log.Info(ctx, "Crypto cert access not found, granting trust",
			slog.String("cert", cert.Name),
			slog.String("subject", subject),
		)
	}

	accessData, err := s.grantCryptoRoleTrust(ctx, subject, cert.Name, ksConfig)
	if err != nil {
		return false, err
	}

	log.Info(ctx, "Crypto role trust granted",
		slog.String("cert", cert.Name),
		slog.String("subject", subject),
	)

	cryptoAccessData[cert.Name] = accessData.Values
	cryptoAccessData[cert.Name][model.CertificateSubjectKey] = subject

	ksConfig.CryptoAccessData[cert.Name] = model.CryptoConfig{
		Subject:    subject,
		AccessData: accessData.Values,
	}

	return true, nil
}

func (s *CryptoAccessDataSyncer) addClientCertToRoleManagementClient(
	ctx context.Context,
	ksConfig *model.KeystoreConfig,
) (map[string]any, error) {
	roleManagementCert, err := s.getRoleManagementCert(ctx)
	if err != nil {
		return nil, err
	}

	configMap := map[string]any{
		"authType":   constants.AuthTypeCertificate,
		"clientCert": roleManagementCert.CertPEM,
		"privateKey": roleManagementCert.PrivateKeyPEM,
	}
	maps.Copy(configMap, ksConfig.RoleManagementConfig.AccessData)

	return configMap, nil
}

func (s *CryptoAccessDataSyncer) grantCryptoRoleTrust(
	ctx context.Context,
	subject, region string,
	ksConfig *model.KeystoreConfig,
) (*common.KeystoreConfig, error) {
	client, err := s.getKeystoreManagementClient()
	if err != nil {
		return nil, err
	}

	configMap, err := s.addClientCertToRoleManagementClient(ctx, ksConfig)
	if err != nil {
		return nil, err
	}

	resp, err := client.GrantTrust(ctx, &keystoremanagement.GrantTrustRequest{
		Config:  common.KeystoreConfig{Values: configMap},
		Subject: subject,
		Region:  region,
	})
	if err != nil {
		return nil, errs.Wrap(errGrantTrustFailed, err)
	}

	return &common.KeystoreConfig{Values: resp.AccessData.Values}, nil
}

func (s *CryptoAccessDataSyncer) removeCryptoRoleTrust(
	ctx context.Context,
	cryptoAccessData map[string]any,
	ksConfig *model.KeystoreConfig,
) error {
	client, err := s.getKeystoreManagementClient()
	if err != nil {
		return err
	}

	configMap, err := s.addClientCertToRoleManagementClient(ctx, ksConfig)
	if err != nil {
		return err
	}

	_, err = client.RemoveTrust(ctx, &keystoremanagement.RemoveTrustRequest{
		Config:     common.KeystoreConfig{Values: configMap},
		AccessData: common.KeystoreConfig{Values: cryptoAccessData},
	})
	if err != nil {
		return errs.Wrap(errRemoveTrustFailed, err)
	}

	return nil
}

func (s *CryptoAccessDataSyncer) getKeystoreManagementClient() (keystoremanagement.KeystoreManagement, error) {
	clients, err := s.svcRegistry.KeystoreManagements()
	if err != nil {
		return nil, errs.Wrap(errPluginCatalogUnavailable, err)
	}

	plugins, err := s.svcRegistry.KeyManagementList()
	if err != nil || len(plugins) == 0 {
		return nil, errs.Wrapf(errPluginCatalogUnavailable, "no keystore plugins found")
	}

	var defaultProvider string
	for _, plugin := range plugins {
		if pluginHelpers.HasTag(plugin.ServiceInfo().Tags(), constants.DefaultKeyStore) {
			defaultProvider = plugin.ServiceInfo().Name()
			break
		}
	}

	if defaultProvider == "" {
		return nil, errs.Wrapf(errPluginCatalogUnavailable, "no default keystore provider found")
	}

	client, ok := clients[defaultProvider]
	if !ok {
		return nil, errs.Wrapf(errPluginCatalogUnavailable, defaultProvider)
	}

	return client, nil
}

func (s *CryptoAccessDataSyncer) getRoleManagementCert(ctx context.Context) (*model.Certificate, error) {
	compositeKey := repo.NewCompositeKey().Where(repo.PurposeField, model.CertificatePurposeRoleManagement)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey)).Order(repo.OrderField{
		Field:     repo.CreationDateField,
		Direction: repo.Desc,
	})

	cert := &model.Certificate{}
	found, err := s.repo.First(ctx, cert, *query)
	if err != nil {
		return nil, errs.Wrap(errGetRoleManagementCert, err)
	}
	if !found {
		return nil, errs.Wrapf(errGetRoleManagementCert, "no role management certificate found")
	}

	return cert, nil
}

func (s *CryptoAccessDataSyncer) getDefaultKeystoreConfig(ctx context.Context) (*model.KeystoreConfig, error) {
	var cfg model.TenantConfig

	ck := repo.NewCompositeKey().Where(repo.KeyField, constants.DefaultKeyStore)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	found, err := s.repo.First(ctx, &cfg, *query)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, errs.Wrap(errGetDefaultKeystoreConfig, err)
	}
	if !found {
		return nil, errs.Wrapf(errGetDefaultKeystoreConfig, "default keystore config not found")
	}

	ksConfig := &model.KeystoreConfig{}
	if err := json.Unmarshal(cfg.Value, ksConfig); err != nil {
		return nil, errs.Wrap(errGetDefaultKeystoreConfig, err)
	}

	return ksConfig, nil
}

func (s *CryptoAccessDataSyncer) setDefaultKeystoreConfig(ctx context.Context, ksConfig *model.KeystoreConfig) error {
	ksBytes, err := json.Marshal(ksConfig)
	if err != nil {
		return errs.Wrap(errSetDefaultKeystoreConfig, err)
	}

	conf := &model.TenantConfig{
		Key:   constants.DefaultKeyStore,
		Value: ksBytes,
	}

	if err := s.repo.Set(ctx, conf); err != nil {
		return errs.Wrap(errSetDefaultKeystoreConfig, err)
	}

	return nil
}

func (s *CryptoAccessDataSyncer) getCryptoCertificates(ctx context.Context) ([]model.ClientCertificate, error) {
	if s.cfg.CryptoLayer.CertX509Trusts.Source == "" {
		return nil, nil
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	bytes, err := commoncfg.LoadValueFromSourceRef(s.cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, errs.Wrap(errLoadCryptoCerts, err)
	}

	var (
		certConfigurations []*config.CryptoCert
		certs              []model.ClientCertificate
	)

	err = yaml.Unmarshal(bytes, &certConfigurations)
	if err != nil {
		return nil, errs.Wrap(errLoadCryptoCerts, err)
	}

	for _, certCfg := range certConfigurations {
		certs = append(certs, model.NewClientCertificateFromConfig(*certCfg, tenantID))
	}

	return certs, nil
}
