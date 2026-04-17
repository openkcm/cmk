package manager

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrCheckKeyConfigManagedByIAMGroups = errors.New("failed to check key configurations managed by IAM groups")
	ErrKeyConfigurationNotAllowed       = errors.New("user has no permission to access key configuration")
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
	user       User
	certs      *CertificateManager
	tagManager Tags
	cmkAuditor *auditor.Auditor
	cfg        *config.Config
}

type KeyConfigFilter struct {
	Expand     bool
	Pagination repo.Pagination
}

func NewKeyConfigManager(
	repository repo.Repo,
	certManager *CertificateManager,
	user User,
	tagManager Tags,
	cmkAuditor *auditor.Auditor,
	cfg *config.Config,
) *KeyConfigManager {
	return &KeyConfigManager{
		repository: repository,
		certs:      certManager,
		user:       user,
		cmkAuditor: cmkAuditor,
		tagManager: tagManager,
		cfg:        cfg,
	}
}

func (m *KeyConfigManager) GetKeyConfigurations(
	ctx context.Context,
	filter KeyConfigFilter,
) ([]*model.KeyConfiguration, int, error) {
	query := getKeyConfigWithTotalsQuery()
	if filter.Expand {
		query.Preload(repo.Preload{"AdminGroup"})
	}

	hasNoGroups, err := m.applyIAMGroupFilter(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	if hasNoGroups {
		// User has no IAM groups - return empty result
		return []*model.KeyConfiguration{}, 0, nil
	}

	return repo.ListAndCount(ctx, m.repository, filter.Pagination, model.KeyConfiguration{}, query)
}

func (m *KeyConfigManager) PostKeyConfigurations(
	ctx context.Context,
	keyConfiguration *model.KeyConfiguration,
) (*model.KeyConfiguration, error) {
	var group model.Group

	exist, err := m.repository.First(
		ctx,
		&group,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where(repo.IDField, keyConfiguration.AdminGroupID))),
	)
	keyConfiguration.AdminGroup = group
	if err != nil || !exist {
		return nil, ErrInvalidKeyAdminGroup
	}

	if group.Role != constants.KeyAdminRole {
		return nil, ErrInvalidKeyAdminGroup
	}

	_, err = m.user.HasKeyConfigAccess(ctx, authz.APIActionCreate, keyConfiguration)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(keyConfiguration.Name) == "" {
		return nil, ErrNameCannotBeEmpty
	}

	err = m.repository.Create(ctx, keyConfiguration)
	if err != nil {
		return nil, errs.Wrap(ErrCreateKeyConfiguration, err)
	}

	return keyConfiguration, nil
}

func (m *KeyConfigManager) DeleteKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
) error {
	keyConfig := &model.KeyConfiguration{ID: keyConfigID}

	_, err := m.user.HasKeyConfigAccess(ctx, authz.APIActionDelete, keyConfig)
	if err != nil {
		return err
	}

	exist, err := repo.HasConnectedSystems(ctx, m.repository, keyConfigID)
	if err != nil {
		return err
	}

	if exist {
		return errs.Wrap(ErrDeleteKeyConfiguration, ErrConnectedSystemToKeyConfig)
	}

	return m.repository.Transaction(ctx, func(ctx context.Context) error {
		_, err = m.repository.Delete(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrDeleteKeyConfiguration, err)
		}

		return m.tagManager.DeleteTags(ctx, keyConfig.ID)
	})
}

func (m *KeyConfigManager) GetKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
) (*model.KeyConfiguration, error) {
	keyConfig := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err := m.user.HasKeyConfigAccess(ctx, authz.APIActionRead, keyConfig)
	if err != nil {
		return nil, err
	}

	query := getKeyConfigWithTotalsQuery().Preload(repo.Preload{"AdminGroup"})
	_, err = m.repository.First(ctx, keyConfig, *query)
	if err != nil {
		return nil, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	return keyConfig, nil
}

func (m *KeyConfigManager) UpdateKeyConfigurationByID(
	ctx context.Context,
	keyConfigID uuid.UUID,
	patchKeyConfig cmkapi.KeyConfigurationPatch,
) (*model.KeyConfiguration, error) {
	keyConfig := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err := m.user.HasKeyConfigAccess(ctx, authz.APIActionUpdate, keyConfig)
	if err != nil {
		return nil, err
	}

	_, err = m.repository.First(
		ctx,
		keyConfig,
		*repo.NewQuery(),
	)
	if err != nil {
		return nil, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if patchKeyConfig.Name != nil && strings.TrimSpace(*patchKeyConfig.Name) == "" {
		return nil, ErrNameCannotBeEmpty
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

type ClientCertificate struct {
	Name    string                   `yaml:"name"`
	RootCA  string                   `yaml:"rootCA"` //nolint:tagliatelle
	Subject ClientCertificateSubject `yaml:"subject"`
}

type ClientCertificateSubject struct {
	Locality           []string `yaml:"locality"`
	OrganizationalUnit []string `yaml:"organizationUnit"` //nolint:tagliatelle
	Organization       []string `yaml:"organization"`
	Country            []string `yaml:"country"`
	CommonNamePrefix   string   `yaml:"commonNamePrefix"`
	CommonName         string
}

func NewClientCertificateSubjectFromPKIX(subject pkix.Name) ClientCertificateSubject {
	return ClientCertificateSubject{
		Locality:           subject.Locality,
		OrganizationalUnit: subject.OrganizationalUnit,
		Organization:       subject.Organization,
		Country:            subject.Country,
		CommonName:         subject.CommonName,
	}
}

// FormatSubjectWithSlashSeparatedOUs transforms the standard X.509 subject string
// to combine multiple OUs with / separator instead of +
func FormatSubjectWithSlashSeparatedOUs(subject ClientCertificateSubject) string {
	s := pkix.Name{
		Locality:           subject.Locality,
		Country:            subject.Country,
		Organization:       subject.Organization,
		OrganizationalUnit: subject.OrganizationalUnit,
		CommonName:         subject.CommonName,
	}
	if len(s.OrganizationalUnit) <= 1 {
		return s.String() // Use standard format if 0 or 1 OU
	}

	// Get standard format
	standardSubject := s.String()

	// Replace OU=X+OU=Y+OU=Z with OU=X/Y/Z
	combinedOU := "OU=" + strings.Join(s.OrganizationalUnit, "/")

	// Build regex to match multiple OU entries
	ouPattern := `OU=[^,+]+((\+OU=[^,+]+)+)`
	re := regexp.MustCompile(ouPattern)

	return re.ReplaceAllString(standardSubject, combinedOU)
}

// GetClientCertificates retrieves the client certificates
func (m *KeyConfigManager) GetClientCertificates(ctx context.Context) (
	map[model.CertificatePurpose][]*ClientCertificate, error,
) {
	tenantDefaultCert, err := m.certs.getDefaultHYOKClientCert(ctx)
	if err != nil {
		return nil, errs.Wrap(ErrGetDefaultCerts, err)
	}

	defaultCerts := []*model.Certificate{tenantDefaultCert}

	clientCerts := make(map[model.CertificatePurpose][]*ClientCertificate)
	clientCerts[model.CertificatePurposeTenantDefault] = make([]*ClientCertificate, len(defaultCerts))

	for i, certificate := range defaultCerts {
		configCert, err := m.transformTenantDefaultCertificate(ctx, certificate.CertPEM,
			m.cfg.Certificates.RootCertURL, ErrGetDefaultCerts)
		if err != nil {
			return nil, err
		}

		clientCerts[model.CertificatePurposeTenantDefault][i] = configCert
	}

	cryptoCerts, err := m.getCryptoCertificates(ctx)
	clientCerts[model.CertificatePurposeCrypto] = cryptoCerts

	if err != nil {
		return nil, err
	}

	return clientCerts, nil
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
		Subject: NewClientCertificateSubjectFromPKIX(cert.Subject),
	}, nil
}

// getCryptoCertificates retrieves crypto certificates from config
func (m *KeyConfigManager) getCryptoCertificates(ctx context.Context) ([]*ClientCertificate, error) {
	bytes, err := commoncfg.LoadValueFromSourceRef(m.cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		return nil, errs.Wrap(ErrLoadCryptoCerts, err)
	}

	var cryptoCerts []*ClientCertificate

	err = yaml.Unmarshal(bytes, &cryptoCerts)
	if err != nil {
		return nil, errs.Wrap(ErrUnmarshalCryptoCerts, err)
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	for _, cert := range cryptoCerts {
		cert.Subject.CommonName = cert.Subject.CommonNamePrefix + tenantID
	}

	return cryptoCerts, nil
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

// applyIAMGroupFilter applies IAM group filtering to the query based on the context.
// Returns true if filtering was applied (and user has no groups), false otherwise.
// System users bypass IAM filtering and can access all key configurations.
func (m *KeyConfigManager) applyIAMGroupFilter(
	ctx context.Context,
	query *repo.Query,
) (bool, error) {
	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		return false, err
	}

	isGroupFiltered, err := m.user.HasKeyConfigAccess(ctx, authz.APIActionRead, nil)
	if err != nil {
		return false, err
	}
	if !isGroupFiltered {
		return false, nil
	}

	// If IAM identifiers list is empty, user has no access
	if len(iamIdentifiers) == 0 {
		return true, nil
	}

	joinCond := repo.JoinCondition{
		Table:     &model.KeyConfiguration{},
		Field:     repo.AdminGroupIDField,
		JoinField: repo.IDField,
		JoinTable: &model.Group{},
	}

	groupTable := (&model.Group{}).TableName()

	// Create query with IAM identifier filter
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf(`"%s".%s`, groupTable, repo.IAMIdField), iamIdentifiers)

	*query = *query.
		Join(repo.InnerJoin, joinCond).
		Where(repo.NewCompositeKeyGroup(ck))

	return false, nil
}
