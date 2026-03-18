package manager

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
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
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	DefaultCertName = "hyok-default"
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
	r            repo.Repo
	user         User
	certs        *CertificateManager
	tagManager   Tags
	cmkAuditor   *auditor.Auditor
	cfg          *config.Config
	eventFactory *eventprocessor.EventFactory
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
	eventFactory *eventprocessor.EventFactory,
	cfg *config.Config,
) *KeyConfigManager {
	return &KeyConfigManager{
		r:            repository,
		certs:        certManager,
		user:         user,
		cmkAuditor:   cmkAuditor,
		tagManager:   tagManager,
		eventFactory: eventFactory,
		cfg:          cfg,
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

	return repo.ListAndCount(ctx, m.r, filter.Pagination, model.KeyConfiguration{}, query)
}

func (m *KeyConfigManager) PostKeyConfigurations(
	ctx context.Context,
	keyConfiguration *model.KeyConfiguration,
) (*model.KeyConfiguration, error) {
	var group model.Group

	exist, err := m.r.First(
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

	err = m.r.Create(ctx, keyConfiguration)
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

	exist, err := repo.HasConnectedSystems(ctx, m.r, keyConfigID)
	if err != nil {
		return err
	}

	if exist {
		return errs.Wrap(ErrDeleteKeyConfiguration, ErrConnectedSystemToKeyConfig)
	}

	return m.r.Transaction(ctx, func(ctx context.Context) error {
		_, err = m.r.Delete(ctx, keyConfig, *repo.NewQuery())
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
	_, err = m.r.First(ctx, keyConfig, *query)
	if err != nil {
		return nil, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	return keyConfig, nil
}

// UpdateKeyConfigurationByID updates a keyconfig
// In case there is an update to the primaryKey invoke system switch events
//
//nolint:cyclop
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

	_, err = m.r.First(
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

	err = m.r.Transaction(ctx, func(ctx context.Context) error {
		if patchKeyConfig.PrimaryKeyID != nil {
			err := m.handleUpdatePrimaryKey(ctx, keyConfig, *patchKeyConfig.PrimaryKeyID)
			if err != nil {
				return errs.Wrap(ErrUpdateKeyConfiguration, err)
			}
			keyConfig.PrimaryKeyID = patchKeyConfig.PrimaryKeyID
		}

		_, err = m.r.Patch(ctx, keyConfig, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrUpdateKeyConfiguration, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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

// Whenever Keyconfig PrimaryKey switches, systems need to send switch events
// If systems had a previous switch event the event key needs to be updated for the retru
func (m *KeyConfigManager) handleUpdatePrimaryKey(
	ctx context.Context,
	keyConfig *model.KeyConfiguration,
	primaryKeyID uuid.UUID,
) error {
	key := &model.Key{ID: primaryKeyID, KeyConfigurationID: keyConfig.ID}
	_, err := m.r.First(ctx, key, *repo.NewQuery())
	if err != nil {
		return err
	}
	if key.State == string(cmkapi.KeyStateDISABLED) {
		return ErrKeyIsNotEnabled
	}

	// Key is valid. If keyconfig has no existing key no need for further validations
	if keyConfig.PrimaryKeyID == nil {
		return nil
	}

	err = m.updatePrimaryKeySystemEvents(
		ctx,
		ptr.GetSafeDeref(keyConfig.PrimaryKeyID).String(),
		primaryKeyID.String(),
	)
	if err != nil {
		return err
	}

	// Send system switches for systems in keyconfig
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(
				repo.KeyConfigIDField, keyConfig.ID),
		),
	)
	return repo.ProcessInBatch(
		ctx,
		m.r,
		query,
		repo.DefaultLimit,
		func(systems []*model.System) error {
			for _, s := range systems {
				_, err := m.eventFactory.SystemSwitchNewPrimaryKey(
					ctx,
					s,
					primaryKeyID.String(),
					keyConfig.PrimaryKeyID.String(),
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}

// updateOldPKeySystemEvents updates keyTo for system event retries
// This can be done as now there is a new primary key and systems
// can only be linked to primary keys, the previous keyTo needs now
// updated the newly set primary key
func (m *KeyConfigManager) updatePrimaryKeySystemEvents(ctx context.Context, oldPkey string, newPkey string) error {
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(
				repo.JSONBField(repo.DataField, "keyIDTo"), oldPkey),
		),
	)
	return repo.ProcessInBatch(ctx, m.r, query, repo.DefaultLimit, func(events []*model.Event) error {
		for _, e := range events {
			systemJobData, err := eventprocessor.GetSystemJobData(e)
			if err != nil {
				return err
			}

			systemJobData.KeyIDTo = newPkey
			bytes, err := json.Marshal(systemJobData)
			if err != nil {
				return err
			}

			e.Data = bytes
			_, err = m.r.Patch(ctx, e, *repo.NewQuery())
			if err != nil {
				return err
			}
		}
		return nil
	})
}
