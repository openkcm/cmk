package manager

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type UserInfo struct {
	Email      string
	FamilyName string
	GivenName  string
	Identifier string
	Role       string
}

type user struct {
	repo       repo.Repo
	cmkAuditor *auditor.Auditor
}

type User interface {
	HasTenantAccess(ctx context.Context) (bool, error)
	HasSystemAccess(ctx context.Context, action authz.Action, system *model.System) (bool, error)
	HasKeyAccess(ctx context.Context, action authz.Action, keyConfig uuid.UUID) (bool, error)
	HasKeyConfigAccess(
		ctx context.Context,
		action authz.Action,
		keyConfig *model.KeyConfiguration,
	) (bool, error)
	GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.Role, error)
	GetUserInfo(ctx context.Context) (UserInfo, error)
	NeedsGroupFiltering(
		ctx context.Context,
		action authz.Action,
		resource authz.ResourceTypeName,
	) (bool, error)
}

func NewUserManager(r repo.Repo, cmkAuditor *auditor.Auditor) User {
	return &user{repo: r, cmkAuditor: cmkAuditor}
}

// NeedsGroupFiltering is used to restrict resource visibility based on user roles, actions and resources.
// Returns true if a resource is restricted to certain roles or users and false if all resources can be viewed
func (u *user) NeedsGroupFiltering(
	ctx context.Context,
	action authz.Action,
	resource authz.ResourceTypeName,
) (bool, error) {
	// System User has access to everything
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return false, nil
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		return true, err
	}

	role, err := u.GetRoleFromIAM(ctx, iamIdentifiers)
	if err != nil {
		return true, err
	}

	if action == authz.ActionRead {
		// Tenant auditor has read-only access to all data
		if role == constants.TenantAuditorRole {
			return false, nil
		}

		// Tenant admin has access to all groups
		if role == constants.TenantAdminRole && resource == authz.ResourceTypeUserGroup {
			return false, nil
		}
	}

	return true, nil
}

// HasKeyAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user in not authorized
func (u *user) HasKeyAccess(
	ctx context.Context,
	action authz.Action,
	keyConfigID uuid.UUID,
) (bool, error) {
	// System User has access to everything
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return false, nil
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action, authz.ResourceTypeKey)
	if err != nil {
		return isGroupFiltered, err
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		&model.KeyConfiguration{ID: keyConfigID},
		action,
		authz.ResourceTypeKey,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.ResourceTypeKey, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

// HasKeyConfigAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user in not authorized
func (u *user) HasKeyConfigAccess(
	ctx context.Context,
	action authz.Action,
	keyConfig *model.KeyConfiguration,
) (bool, error) {
	// System User has access to everything
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return false, nil
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action, authz.ResourceTypeKeyConfiguration)
	if err != nil {
		return isGroupFiltered, err
	}

	if keyConfig == nil {
		// No keyconfig is being accessed, just checking for visibility scope
		if action == authz.ActionRead {
			return isGroupFiltered, nil
		}
		return isGroupFiltered, ErrKeyConfigurationNotFound
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		keyConfig,
		action,
		authz.ResourceTypeKeyConfiguration,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.ResourceTypeKeyConfiguration, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

// HasSystemAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user in not authorized
func (u *user) HasSystemAccess(
	ctx context.Context,
	action authz.Action,
	system *model.System,
) (bool, error) {
	// System User has access to everything
	isSystemUser := cmkcontext.IsSystemUser(ctx)
	if isSystemUser {
		return false, nil
	}

	// System not linked to any key config, accessible to all users
	if system.KeyConfigurationID == nil {
		return false, nil
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action, authz.ResourceTypeSystem)
	if err != nil {
		return isGroupFiltered, err
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		&model.KeyConfiguration{ID: *system.KeyConfigurationID},
		action,
		authz.ResourceTypeSystem,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.ResourceTypeSystem, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

func (u *user) GetUserInfo(ctx context.Context) (UserInfo, error) {
	clientData, err := cmkcontext.ExtractClientData(ctx)
	if err != nil {
		return UserInfo{}, err
	}

	groups, err := cmkcontext.ExtractClientDataGroups(ctx)
	if err != nil {
		return UserInfo{}, err
	}

	role, err := u.GetRoleFromIAM(ctx, groups)
	if err != nil {
		return UserInfo{}, err
	}

	return UserInfo{
		Identifier: clientData.Identifier,
		Email:      clientData.Email,
		GivenName:  clientData.GivenName,
		FamilyName: clientData.FamilyName,
		Role:       string(role),
	}, nil
}

func (u *user) HasTenantAccess(ctx context.Context) (bool, error) {
	iamIdentifiers, err := cmkcontext.ExtractClientDataGroups(ctx)
	if err != nil {
		return false, ErrTenantNotAllowed
	}

	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	var groups []model.Group

	count, err := u.repo.List(
		ctx, &model.Group{}, &groups,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).SetLimit(0),
	)
	if err != nil {
		return false, errs.Wrap(ErrCheckTenantHasIAMGroups, err)
	}

	return count > 0, nil
}

func (u *user) GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.Role, error) {
	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	var groups []model.Group

	count, err := u.repo.List(
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

	if len(roleMap) > 1 {
		return "", ErrMultipleRolesInGroups
	}

	for k := range roleMap {
		return k, nil
	}

	return "", ErrZeroRolesInGroups
}

func (u *user) isCreateKeyconfig(
	keyConfig *model.KeyConfiguration,
	action authz.Action,
	resource authz.ResourceTypeName,
) bool {
	return action == authz.ActionCreate &&
		resource == authz.ResourceTypeKeyConfiguration &&
		(keyConfig.AdminGroup != model.Group{})
}

// hasKeyConfigAccess checks if a specific key configuration is managed by the user
// IAM groups
func (u *user) hasKeyConfigAccess(
	ctx context.Context,
	keyConfig *model.KeyConfiguration,
	action authz.Action,
	resource authz.ResourceTypeName,
) (bool, error) {
	iamIdentifiers, err := cmkcontext.ExtractClientDataGroupsString(ctx)
	if err != nil {
		return false, err
	}

	// If no IAM identifiers provided, user cannot be authorized through IAM groups
	if len(iamIdentifiers) == 0 {
		return false, ErrKeyConfigurationNotAllowed
	}

	if u.isCreateKeyconfig(keyConfig, action, resource) {
		isAuthorized := slices.Contains(iamIdentifiers, keyConfig.AdminGroup.IAMIdentifier)
		return isAuthorized, nil
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
		Where(fmt.Sprintf(`"%s".%s`, keyConfigTable, repo.IDField), keyConfig.ID).
		Where(fmt.Sprintf(`"%s".%s`, groupTable, repo.IAMIdField), iamIdentifiers)

	query := *repo.NewQuery().
		Join(repo.InnerJoin, joinCond).
		Where(repo.NewCompositeKeyGroup(ck)).
		SetLimit(0)

	count, err := u.repo.Count(ctx, &model.KeyConfiguration{}, query)
	if err != nil {
		return false, errs.Wrap(ErrCheckKeyConfigManagedByIAMGroups, err)
	}

	return count > 0, nil
}

func (u *user) sendUnauthorizedAccessAuditLog(
	ctx context.Context,
	resource authz.ResourceTypeName,
	action authz.Action,
) {
	err := u.cmkAuditor.SendCmkUnauthorizedRequestAuditLog(ctx, string(resource), string(action))
	if err != nil {
		log.Error(ctx, "Failed to send unauthorized access audit log", err)
	}

	log.Info(ctx, "Sent unauthorized access audit log")
}
