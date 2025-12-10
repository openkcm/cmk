package manager

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	idmv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
	"github.tools.sap/kms/cmk/utils/ptr"
)

type GroupManager struct {
	repo    repo.Repo
	catalog *plugincatalog.Catalog
}

func NewGroupManager(
	repository repo.Repo,
	catalog *plugincatalog.Catalog,
) *GroupManager {
	return &GroupManager{
		repo:    repository,
		catalog: catalog,
	}
}

type GroupIAMExistence struct {
	IAMIdentifier string
	Exists        bool
}

func (m *GroupManager) GetGroups(ctx context.Context, skip int, top int) ([]*model.Group, int, error) {
	// Check if user has any groups (system users always have access)
	hasAccess := m.hasAccessToGroups(ctx)

	if !hasAccess {
		return []*model.Group{}, 0, nil
	}

	query := repo.NewQuery().SetLimit(top).SetOffset(skip)

	m.applyIAMGroupFilter(ctx, query)

	var groups []*model.Group
	count, err := m.repo.List(
		ctx, model.Group{}, &groups, *query,
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrListGroups, err)
	}

	return groups, count, nil
}

func (m *GroupManager) CreateGroup(ctx context.Context, group *model.Group) (*model.Group, error) {
	if !m.isSupportedRole(group) {
		return nil, ErrGroupRole
	}

	err := m.repo.Create(ctx, group)
	if err != nil {
		return nil, errs.Wrap(ErrCreateGroups, err)
	}

	return group, nil
}

func (m *GroupManager) DeleteGroupByID(ctx context.Context, id uuid.UUID) error {
	group, err := m.GetGroupByID(ctx, id)
	if err != nil {
		return errs.Wrap(ErrDeleteGroups, err)
	}

	if m.isMandatoryGroup(group) {
		return ErrInvalidGroupDelete
	}

	keyConfig := &model.KeyConfiguration{}
	exist, err := m.repo.First(
		ctx,
		keyConfig,
		*repo.NewQuery().
			Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(repo.AdminGroupIDField, id),
				),
			),
	)

	if exist {
		return ErrInvalidGroupDelete
	}

	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return errs.Wrap(ErrGetGroups, err)
	}

	_, err = m.repo.Delete(ctx, &model.Group{ID: id}, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrDeleteGroups, err)
	}

	return nil
}

func (m *GroupManager) GetGroupByID(ctx context.Context, id uuid.UUID) (*model.Group, error) {
	// Check if user has access (system users always have access)
	hasAccess := m.hasAccessToGroups(ctx)

	if !hasAccess {
		return nil, errs.Wrap(ErrGetGroups, repo.ErrNotFound)
	}

	query := repo.NewQuery()
	m.applyIAMGroupFilter(ctx, query)

	group := &model.Group{ID: id}
	_, err := m.repo.First(ctx, group, *query)
	if err != nil {
		return nil, errs.Wrap(ErrGetGroups, err)
	}

	return group, nil
}

func (m *GroupManager) UpdateGroup(
	ctx context.Context,
	id uuid.UUID,
	patchGroup cmkapi.GroupPatch,
) (*model.Group, error) {
	group, err := m.GetGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !ptr.IsValidStrPtr(patchGroup.Name) {
		return nil, errs.Wrap(ErrNameCannotBeEmpty, nil)
	}

	if m.isMandatoryGroup(group) || m.isReservedName(patchGroup) {
		return nil, ErrInvalidGroupRename
	}

	if patchGroup.Name != nil {
		group.Name = *patchGroup.Name

		tenantID, err := cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return nil, err
		}

		group.IAMIdentifier = model.NewIAMIdentifier(group.Name, tenantID)
	}

	if patchGroup.Description != nil {
		group.Description = *patchGroup.Description
	}

	_, err = m.repo.Patch(ctx, group, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrUpdateGroups, err)
	}

	return group, nil
}

// CreateDefaultGroups creates the default admin and auditor groups for a tenant.
func (m *GroupManager) CreateDefaultGroups(ctx context.Context) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return errs.Wrap(ErrCreateGroups, err)
	}

	iamAdmin := model.NewIAMIdentifier(constants.TenantAdminGroup, tenantID)

	iamAuditor := model.NewIAMIdentifier(constants.TenantAuditorGroup, tenantID)

	err = m.repo.Transaction(
		ctx, func(ctx context.Context, _ repo.Repo) error {
			_, err := m.CreateGroup(
				ctx, &model.Group{
					ID:            uuid.New(),
					Name:          constants.TenantAdminGroup,
					Role:          constants.TenantAdminRole,
					IAMIdentifier: iamAdmin,
				},
			)
			if err != nil {
				if errors.Is(err, repo.ErrUniqueConstraint) {
					err = errs.Wrap(ErrOnboardingInProgress, err)
				}

				return errs.Wrap(ErrCreatingGroups, err)
			}

			_, err = m.CreateGroup(
				ctx, &model.Group{
					ID:            uuid.New(),
					Name:          constants.TenantAuditorGroup,
					Role:          constants.TenantAuditorRole,
					IAMIdentifier: iamAuditor,
				},
			)
			if err != nil {
				if errors.Is(err, repo.ErrUniqueConstraint) {
					err = errs.Wrap(ErrOnboardingInProgress, err)
				}

				return errs.Wrap(ErrCreatingGroups, err)
			}

			return nil
		},
	)

	return err
}

// BuildIAMIdentifier creates an IAM identifier for a group based on its type and tenant ID.
func (m *GroupManager) BuildIAMIdentifier(groupType, tenantID string) (string, error) {
	if tenantID == "" {
		return "", ErrEmptyTenantID
	}

	if groupType != constants.TenantAdminGroup && groupType != constants.TenantAuditorGroup {
		return "", ErrInvalidGroupType
	}

	return model.NewIAMIdentifier(groupType, tenantID), nil
}

func (m *GroupManager) GetIdentityManagementPlugin() (idmv1.IdentityManagementServiceClient, error) {
	if m.catalog == nil {
		return nil, errs.Wrapf(ErrLoadIdentityManagementPlugin, "plugin catalog is not initialized")
	}

	plugins := m.catalog.LookupByType(idmv1.Type)
	if len(plugins) == 0 {
		return nil, errs.Wrapf(ErrLoadIdentityManagementPlugin, "no identity management plugins found in catalog")
	}

	if len(plugins) > 1 {
		return nil, errs.Wrapf(ErrLoadIdentityManagementPlugin, "multiple identity management plugins found in catalog")
	}

	connection := plugins[0].ClientConnection()
	client := idmv1.NewIdentityManagementServiceClient(connection)

	return client, nil
}

func (m *GroupManager) CheckIAMExistenceOfGroups(
	ctx context.Context,
	iamIdentifiers []string,
) ([]GroupIAMExistence, error) {
	client, err := m.GetIdentityManagementPlugin()
	if err != nil {
		return nil, err
	}

	result := make([]GroupIAMExistence, 0, len(iamIdentifiers))
	for _, name := range iamIdentifiers {
		request := &idmv1.GetGroupRequest{
			GroupName: name,
		}

		_, err := client.GetGroup(ctx, request)
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.NotFound {
				result = append(
					result, GroupIAMExistence{
						IAMIdentifier: name,
						Exists:        false,
					},
				)

				continue
			}

			return nil, errs.Wrap(ErrCheckIAMExistenceOfGroups, err)
		}

		result = append(
			result, GroupIAMExistence{
				IAMIdentifier: name,
				Exists:        true,
			},
		)
	}

	return result, nil
}

// CheckTenantHasAnyIAMGroups checks if any of the provided IAM group identifiers exist
// in the database.
func (m *GroupManager) CheckTenantHasAnyIAMGroups(
	ctx context.Context,
	iamIdentifiers []constants.UserGroup,
) (bool, error) {
	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	var groups []model.Group

	count, err := m.repo.List(
		ctx, &model.Group{}, &groups,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).SetLimit(0),
	)
	if err != nil {
		return false, errs.Wrap(ErrCheckTenantHasIAMGroups, err)
	}

	return count > 0, nil
}

func (m *GroupManager) GetRoleFromGroupIAMIdentifiers(
	ctx context.Context,
	iamIdentifiers []constants.UserGroup,
) (constants.Role, error) {
	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	var groups []model.Group

	count, err := m.repo.List(
		ctx, &model.Group{}, &groups,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return "", errs.Wrap(ErrGetGroups, err)
	}

	if count == 0 {
		return "", nil
	}

	roleMap := map[constants.Role]bool{}
	for _, group := range groups {
		roleMap[group.Role] = true
	}

	roles := make([]constants.Role, 0, len(roleMap))
	for role := range roleMap {
		roles = append(roles, role)
	}

	if len(roles) > 1 {
		return "", ErrMultipleRolesInGroups
	}

	return roles[0], nil
}

func (m *GroupManager) isMandatoryGroup(group *model.Group) bool {
	return group.Name == constants.TenantAdminGroup || group.Name == constants.TenantAuditorGroup
}

func (m *GroupManager) isReservedName(patchGroup cmkapi.GroupPatch) bool {
	if patchGroup.Name == nil {
		return false
	}

	return m.isMandatoryGroup(&model.Group{Name: *patchGroup.Name})
}

func (m *GroupManager) isSupportedRole(group *model.Group) bool {
	switch group.Role {
	case constants.TenantAdminRole, constants.TenantAuditorRole, constants.KeyAdminRole:
		return true
	default:
		return false
	}
}

// hasAccessToGroups checks if user has access to groups.
// System users always have access. Regular users must have at least one IAM group assigned.
// Returns false if user has no groups (should see empty results).
func (m *GroupManager) hasAccessToGroups(ctx context.Context) bool {
	// System users have access to all groups
	if cmkcontext.IsSystemUser(ctx) {
		return true
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		log.Error(ctx, "failed to extract client data groups: %v", err)
		return false
	}
	return len(iamIdentifiers) > 0
}

// isTenantAdmin checks if user has TenantAdmin role.
func (m *GroupManager) isTenantAdmin(ctx context.Context, iamIdentifiers []string) (bool, error) {
	role, err := m.GetRoleFromGroupIAMIdentifiers(ctx, convertToUserGroups(iamIdentifiers))
	if err != nil {
		return false, err
	}
	return role == constants.TenantAdminRole, nil
}

// applyIAMGroupFilter adds IAM filtering to query if user is not TenantAdmin.
// SystemUser bypass filtering completely.
// TenantAdmins see all groups, others only see their own groups.
func (m *GroupManager) applyIAMGroupFilter(ctx context.Context, query *repo.Query) {
	// System users bypass all filtering
	if cmkcontext.IsSystemUser(ctx) {
		return
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		log.Error(ctx, "failed to extract client data groups: %v", err)
	}

	isTenantAdmin, err := m.isTenantAdmin(ctx, iamIdentifiers)
	if err != nil {
		log.Error(ctx, "failed to determine if user is TenantAdmin: %v", err)
	}

	if isTenantAdmin {
		return
	}
	// Non-admins only see their own groups
	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)
	*query = *query.Where(repo.NewCompositeKeyGroup(ck))
}

// convertToUserGroups converts string slice to UserGroup slice
func convertToUserGroups(identifiers []string) []constants.UserGroup {
	result := make([]constants.UserGroup, len(identifiers))
	for i, id := range identifiers {
		result[i] = constants.UserGroup(id)
	}
	return result
}
