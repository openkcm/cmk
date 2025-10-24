package manager

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
)

const (
	DefaultCertName = "hyok-default"
)

var (
	ErrGetDefaultCerts = errors.New("failed to get default certificates")
	ErrDecodingCert    = errors.New("failed to decode certificate")
)

type KeyConfigurationAPI interface {
	GetKeyConfigurations(ctx context.Context, filter KeyConfigFilter) ([]*model.KeyConfiguration, int, error)
	PostKeyConfigurations(ctx context.Context, key *model.KeyConfiguration) (*model.KeyConfiguration, error)
	DeleteKeyConfigurationByID(ctx context.Context, keyConfigID uuid.UUID) error
	GetKeyConfigurationByID(ctx context.Context, keyConfigID uuid.UUID) (*model.KeyConfiguration, error)
	UpdateKeyConfigurationByID(
		ctx context.Context,
		keyConfigID uuid.UUID,
		patchKeyConfig cmkapi.KeyConfigurationPatch,
	) (*model.KeyConfiguration, error)
	GetClientCertificates(ctx context.Context) (map[model.CertificatePurpose][]*ClientCertificate, error)
}

type KeyConfigManager struct {
	repository repo.Repo
	certs      *CertificateManager
	cfg        *config.Config
}

type KeyConfigFilter struct {
	Expand bool
	Skip   int
	Top    int
}

func NewKeyConfigManager(
	repository repo.Repo,
	certManager *CertificateManager,
	cfg *config.Config,
) *KeyConfigManager {
	return &KeyConfigManager{
		repository: repository,
		certs:      certManager,
		cfg:        cfg,
	}
}

func (m *KeyConfigManager) GetKeyConfigurations(
	ctx context.Context,
	filter KeyConfigFilter,
) ([]*model.KeyConfiguration, int, error) {
	var res []*model.KeyConfiguration

	query := getKeyConfigWithTotalsQuery().SetLimit(filter.Top).SetOffset(filter.Skip)
	if filter.Expand {
		query.Preload(repo.Preload{"AdminGroup"})
	}

	count, err := m.repository.List(
		ctx,
		model.KeyConfiguration{},
		&res,
		*query,
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrQueryKeyConfigurationList, err)
	}

	return res, count, nil
}

func (m *KeyConfigManager) PostKeyConfigurations(
	ctx context.Context,
	key *model.KeyConfiguration,
) (*model.KeyConfiguration, error) {
	exist, err := m.repository.First(
		ctx,
		&model.Group{},
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where(repo.IDField, key.AdminGroupID))),
	)
	if err != nil || !exist {
		return nil, ErrInvalidKeyAdminGroup
	}

	err = m.repository.Create(ctx, key)
	if err != nil {
		return nil, errs.Wrap(ErrCreateKeyConfiguration, err)
	}

	return key, nil
}

func (m *KeyConfigManager) DeleteKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
) error {
	keyConfig := &model.KeyConfiguration{ID: keyConfigID}

	exist, err := repo.HasConnectedSystems(ctx, m.repository, keyConfigID)
	if err != nil {
		return err
	}

	if exist {
		return errs.Wrap(ErrDeleteKeyConfiguration, ErrConnectedSystemToKeyConfig)
	}

	_, err = m.repository.Delete(ctx, keyConfig, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrDeleteKeyConfiguration, err)
	}

	return nil
}

func (m *KeyConfigManager) GetKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
) (*model.KeyConfiguration, error) {
	item := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err := m.repository.First(ctx, item, *getKeyConfigWithTotalsQuery().Preload(repo.Preload{"Tags"}))
	if err != nil {
		return nil, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	return item, nil
}

func (m *KeyConfigManager) UpdateKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
	patchKeyConfig cmkapi.KeyConfigurationPatch,
) (*model.KeyConfiguration, error) {
	keyConfig := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err := m.repository.First(
		ctx,
		keyConfig,
		*repo.NewQuery().Preload(repo.Preload{"Tags"}),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if patchKeyConfig.Name != nil && *patchKeyConfig.Name == "" {
		return nil, errs.Wrap(ErrNameCannotBeEmpty, nil)
	}

	if patchKeyConfig.Name != nil {
		keyConfig.Name = *patchKeyConfig.Name
	}

	if patchKeyConfig.Description != nil {
		keyConfig.Description = *patchKeyConfig.Description
	}

	_, err = m.repository.Patch(ctx, keyConfig, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrUpdateKeyConfiguration, err)
	}

	return keyConfig, nil
}

// GetClientCertificates retrieves the client certificates
func (m *KeyConfigManager) GetClientCertificates(ctx context.Context) (
	map[model.CertificatePurpose][]*ClientCertificate, error,
) {
	certConfig := m.certs.cfg

	tenantDefaultCert, err := m.certs.getDefaultHYOKClientCert(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultCerts, err)
	}

	defaultCerts := []*model.Certificate{tenantDefaultCert}

	clientCerts := make(map[model.CertificatePurpose][]*ClientCertificate)
	clientCerts[model.CertificatePurposeTenantDefault] = make([]*ClientCertificate,
		len(defaultCerts))

	for i, certificate := range defaultCerts {
		configCert, err := m.transformTenantDefaultCertificate(ctx, certificate.CertPEM,
			certConfig.RootCertURL, ErrGetDefaultCerts)
		if err != nil {
			return nil, err
		}

		clientCerts[model.CertificatePurposeTenantDefault][i] = configCert
	}

	cryptoCerts, err := m.getCryptoCertificates()
	clientCerts[model.CertificatePurposeCrypto] = cryptoCerts

	if err != nil {
		return nil, err
	}

	return clientCerts, nil
}

// ClientCertificate represents the client certificates
type ClientCertificate struct {
	Name    string
	RootCA  string
	Subject string
}

func (m *KeyConfigManager) transformTenantDefaultCertificate(_ context.Context,
	certRaw, rootCertURL string, errParent error,
) (*ClientCertificate, error) {
	block, _ := pem.Decode([]byte(certRaw))
	if block == nil {
		return nil, errs.Wrap(errParent, ErrDecodingCert)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errs.Wrap(errParent, err)
	}

	return &ClientCertificate{
		Name:    DefaultCertName,
		RootCA:  rootCertURL,
		Subject: cert.Subject.String(),
	}, nil
}

// getCryptoCertificates retrieves crypto certificates from config
func (m *KeyConfigManager) getCryptoCertificates() ([]*ClientCertificate, error) {
	bytes, err := commoncfg.LoadValueFromSourceRef(m.cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, errs.Wrap(ErrLoadCryptoCerts, err)
	}

	var cryptoCerts map[string]ClientCertificate

	err = json.Unmarshal(bytes, &cryptoCerts)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalCryptoCerts, err)
	}

	return m.certMapToSlice(cryptoCerts), nil
}

func (m *KeyConfigManager) certMapToSlice(certs map[string]ClientCertificate) []*ClientCertificate {
	l := make([]*ClientCertificate, 0, len(certs))
	for k, v := range certs {
		l = append(l, &ClientCertificate{
			Name:    k,
			Subject: v.Subject,
			RootCA:  v.RootCA,
		})
	}

	return l
}

func getKeyConfigWithTotalsQuery() *repo.Query {
	return repo.NewQueryWithFieldLoading(
		model.KeyConfiguration{},
		repo.LoadingFields{
			Table:     model.System{},
			JoinField: repo.KeyConfigIDField,
			SelectField: repo.SelectField{
				Field: repo.IDField,
				Func: repo.QueryFunction{
					Function: repo.CountFunc,
					Distinct: true,
				},
				Alias: repo.KeyconfigTotalSystems,
			},
		},
		repo.LoadingFields{
			Table:     model.Key{},
			JoinField: repo.KeyConfigIDField,
			SelectField: repo.SelectField{
				Field: repo.IDField,
				Func: repo.QueryFunction{
					Function: repo.CountFunc,
					Distinct: true,
				},
				Alias: repo.KeyconfigTotalKeys,
			},
		},
	)
}
