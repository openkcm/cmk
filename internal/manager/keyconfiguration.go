package manager

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/authz"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

const (
	DefaultCertName = "hyok-default"
)

var (
	ErrGetDefaultCerts                  = errors.New("failed to get default certificates")
	ErrDecodingCert                     = errors.New("failed to decode certificate")
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
	certs      *CertificateManager
	cmkAuditor *auditor.Auditor
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
	cmkAuditor *auditor.Auditor,
	cfg *config.Config,
) *KeyConfigManager {
	return &KeyConfigManager{
		repository: repository,
		certs:      certManager,
		cmkAuditor: cmkAuditor,
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

	hasNoGroups, err := m.applyIAMGroupFilter(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	if hasNoGroups {
		// User has no IAM groups - return empty result
		return []*model.KeyConfiguration{}, 0, nil
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
	if err != nil || !exist {
		return nil, ErrInvalidKeyAdminGroup
	}

	if group.Role != constants.KeyAdminRole {
		return nil, ErrInvalidKeyAdminGroup
	}

	err = m.CheckKeyConfigGroupMembershipForPost(ctx, group)
	if err != nil {
		return nil, err
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
	err := m.CheckKeyConfigGroupMembership(ctx, keyConfigID)
	if err != nil {
		return err
	}

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
	err := m.CheckKeyConfigGroupMembership(ctx, keyConfigID)
	if err != nil {
		return nil, err
	}

	item := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err = m.repository.First(ctx, item, *getKeyConfigWithTotalsQuery().Preload(repo.Preload{"Tags"}))
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
	err := m.CheckKeyConfigGroupMembership(ctx, keyConfigID)
	if err != nil {
		return nil, err
	}

	keyConfig := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err = m.repository.First(
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

// CheckKeyConfigGroupMembership verifies if the user has access to a key configuration
// based on their IAM group membership. Returns an error if authorization fails.
func (m *KeyConfigManager) CheckKeyConfigGroupMembership(
	ctx context.Context,
	keyConfigID uuid.UUID,
) error {
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	// If the user is a system user, they have access to all key configurations
	if isSystemUser {
		return nil
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		return err
	}

	// Check if the key configuration is managed by one of the user's IAM groups
	isAuthorized, inErr := m.isKeyConfigManagedByIAMGroups(ctx, keyConfigID, iamIdentifiers)
	if inErr != nil {
		return errs.Wrap(ErrGettingKeyConfigByID, inErr)
	}

	if !isAuthorized {
		m.sendUnauthorizedAccessAuditLog(ctx, string(authz.ResourceTypeKeyConfiguration), string(authz.ActionRead))
		return ErrKeyConfigurationNotAllowed
	}

	return nil
}

// CheckKeyConfigGroupMembershipForPost verifies if the user has access to a key configuration
// based on their IAM group membership for POST operations. Returns an error if authorization fails.
func (m *KeyConfigManager) CheckKeyConfigGroupMembershipForPost(
	ctx context.Context,
	group model.Group,
) error {
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return nil
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		return err
	}

	isAuthorized := slices.Contains(iamIdentifiers, group.IAMIdentifier)
	if !isAuthorized {
		m.sendUnauthorizedAccessAuditLog(ctx, string(authz.ResourceTypeKeyConfiguration), string(authz.ActionCreate))
		return ErrKeyConfigurationNotAllowed
	}

	return nil
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

	subject := formatSubjectWithSlashSeparatedOUs(cert.Subject)

	return &ClientCertificate{
		Name:    DefaultCertName,
		RootCA:  rootCertURL,
		Subject: subject,
	}, nil
}

// formatSubjectWithSlashSeparatedOUs transforms the standard X.509 subject string
// to combine multiple OUs with / separator instead of +
func formatSubjectWithSlashSeparatedOUs(subject pkix.Name) string {
	if len(subject.OrganizationalUnit) <= 1 {
		return subject.String() // Use standard format if 0 or 1 OU
	}

	// Get standard format
	standardSubject := subject.String()

	// Replace OU=X+OU=Y+OU=Z with OU=X/Y/Z
	combinedOU := "OU=" + strings.Join(subject.OrganizationalUnit, "/")

	// Build regex to match multiple OU entries
	ouPattern := `OU=[^,+]+((\+OU=[^,+]+)+)`
	re := regexp.MustCompile(ouPattern)

	return re.ReplaceAllString(standardSubject, combinedOU)
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

// applyIAMGroupFilter applies IAM group filtering to the query based on the context.
// Returns true if filtering was applied (and user has no groups), false otherwise.
// System users bypass IAM filtering and can access all key configurations.
//
//nolint:unparam,nilerr
func (m *KeyConfigManager) applyIAMGroupFilter(
	ctx context.Context,
	query *repo.Query,
) (bool, error) {
	// System users have access to all key configurations
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return false, nil
	}

	// Backward compatibility: if we cannot extract IAM identifiers, do not apply filtering.
	// Need to change this behavior after ClientHeader authorization check is in placed.
	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
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

// isKeyConfigManagedByIAMGroups checks if a specific key configuration is managed by groups
// with any of the provided IAM group identifiers.
func (m *KeyConfigManager) isKeyConfigManagedByIAMGroups(
	ctx context.Context,
	keyConfigID uuid.UUID,
	iamIdentifiers []string,
) (bool, error) {
	// If no IAM identifiers provided, user cannot be authorized through IAM groups
	if len(iamIdentifiers) == 0 {
		return false, nil
	}

	joinCond := repo.JoinCondition{
		Table:     &model.KeyConfiguration{},
		Field:     repo.AdminGroupIDField,
		JoinField: repo.IDField,
		JoinTable: &model.Group{},
	}

	keyConfigTable := (&model.KeyConfiguration{}).TableName()
	groupTable := (&model.Group{}).TableName()

	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf(`"%s".%s`, keyConfigTable, repo.IDField), keyConfigID).
		Where(fmt.Sprintf(`"%s".%s`, groupTable, repo.IAMIdField), iamIdentifiers)

	query := *repo.NewQuery().
		Join(repo.InnerJoin, joinCond).
		Where(repo.NewCompositeKeyGroup(ck)).
		SetLimit(0)

	count, err := m.repository.Count(ctx, &model.KeyConfiguration{}, query)
	if err != nil {
		return false, errs.Wrap(ErrCheckKeyConfigManagedByIAMGroups, err)
	}

	return count > 0, nil
}

func (m *KeyConfigManager) sendUnauthorizedAccessAuditLog(ctx context.Context, resource, action string) {
	err := m.cmkAuditor.SendCmkUnauthorizedRequestAuditLog(ctx, resource, action)
	if err != nil {
		log.Error(ctx, "Failed to send unauthorized access audit log", err)
	}

	log.Info(ctx, "Sent unauthorized access audit log")
}
