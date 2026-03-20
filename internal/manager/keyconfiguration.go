package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkContext "github.com/openkcm/cmk/utils/context"
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
}

type KeyConfigManager struct {
	repository repo.Repo
	user       User
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
	user User,
	tagManager Tags,
	cmkAuditor *auditor.Auditor,
	cfg *config.Config,
) *KeyConfigManager {
	return &KeyConfigManager{
		repository: repository,
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
	iamIdentifiers, err := cmkContext.ExtractClientDataGroupsString(ctx)
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
