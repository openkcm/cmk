package manager

import (
	"context"
	"errors"
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

type BusinessUserInfo struct {
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
	HasSystemAccess(ctx context.Context, action authz.APIAction, system *model.System) (bool, error)
	HasKeyAccess(ctx context.Context, action authz.APIAction, keyConfig uuid.UUID) (bool, error)
	HasKeyConfigAccess(
		ctx context.Context,
		action authz.APIAction,
		keyConfig *model.KeyConfiguration,
	) (bool, error)
	GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.BusinessRole, error)
	GetBusinessUserInfo(ctx context.Context) (BusinessUserInfo, error)
	NeedsGroupFiltering(
		ctx context.Context,
		action authz.APIAction,
		resource authz.APIResourceType,
	) (bool, error)
}

func NewUserManager(
	r repo.Repo,
	cmkAuditor *auditor.Auditor,
) User {
	return &user{
		repo:       r,
		cmkAuditor: cmkAuditor,
	}
}

// NeedsGroupFiltering is used to restrict resource visibility based on user roles, actions and resources.
// Returns true if a resource is restricted to certain roles or users and false if all resources can be viewed.
func (u *user) NeedsGroupFiltering(
	ctx context.Context,
	action authz.APIAction,
	resource authz.APIResourceType,
) (bool, error) {
	err := ensureBusinessUserOpsAllowed(ctx, []constants.InternalRole{
		constants.InternalTaskWorkflowApproversRole,
		constants.InternalTaskWorkflowExpirationRole,
		constants.InternalTenantProvisioningRole,
	})
	if errors.Is(err, errInternalBypass) {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	iamIdentifiers, err := cmkcontext.ExtractBusinessUserDataGroupsString(ctx)
	if err != nil {
		return true, err
	}

	role, err := u.GetRoleFromIAM(ctx, iamIdentifiers)
	if err != nil {
		return true, err
	}

	if action == authz.APIActionRead && u.roleBypassesGroupFilter(role, resource) {
		return false, nil
	}

	return true, nil
}

// HasKeyAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user is not authorized
func (u *user) HasKeyAccess(
	ctx context.Context,
	action authz.APIAction,
	keyConfigID uuid.UUID,
) (bool, error) {
	err := ensureBusinessUserOpsAllowed(ctx, []constants.InternalRole{
		constants.InternalTaskWorkflowApproversRole,
		constants.InternalTenantProvisioningRole,
	})
	if errors.Is(err, errInternalBypass) {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action, authz.APIResourceTypeKey)
	if err != nil {
		return isGroupFiltered, err
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		&model.KeyConfiguration{ID: keyConfigID},
		action,
		authz.APIResourceTypeKey,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.APIResourceTypeKey, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

// HasKeyConfigAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user in not authorized
func (u *user) HasKeyConfigAccess(
	ctx context.Context,
	action authz.APIAction,
	keyConfig *model.KeyConfiguration,
) (bool, error) {
	err := ensureBusinessUserOpsAllowed(ctx, []constants.InternalRole{
		constants.InternalTaskWorkflowApproversRole,
		constants.InternalTenantProvisioningRole,
	})
	if errors.Is(err, errInternalBypass) {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action,
		authz.APIResourceTypeKeyConfiguration)
	if err != nil {
		return isGroupFiltered, err
	}

	if keyConfig == nil {
		// No keyconfig is being accessed, just checking for visibility scope
		if action == authz.APIActionRead {
			return isGroupFiltered, nil
		}
		return isGroupFiltered, ErrKeyConfigurationNotFound
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		keyConfig,
		action,
		authz.APIResourceTypeKeyConfiguration,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.APIResourceTypeKeyConfiguration, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

// HasSystemAccess checks if a user can execute operations on the resource
// It returns true if it's group restricted and errors if the user in not authorized
func (u *user) HasSystemAccess(
	ctx context.Context,
	action authz.APIAction,
	system *model.System,
) (bool, error) {
	err := ensureBusinessUserOpsAllowed(ctx, []constants.InternalRole{
		constants.InternalTaskWorkflowApproversRole,
		constants.InternalTenantProvisioningRole,
	})
	if errors.Is(err, errInternalBypass) {
		return false, nil
	}
	if err != nil {
		return true, err
	}

	// System not linked to any key config, accessible to all users
	if system.KeyConfigurationID == nil {
		return false, nil
	}

	isGroupFiltered, err := u.NeedsGroupFiltering(ctx, action, authz.APIResourceTypeSystem)
	if err != nil {
		return isGroupFiltered, err
	}

	isAuthorized, err := u.hasKeyConfigAccess(
		ctx,
		&model.KeyConfiguration{ID: *system.KeyConfigurationID},
		action,
		authz.APIResourceTypeSystem,
	)
	if err != nil {
		return isGroupFiltered, errs.Wrap(ErrGettingKeyConfigByID, err)
	}

	if !isAuthorized {
		u.sendUnauthorizedAccessAuditLog(ctx, authz.APIResourceTypeSystem, action)
		return isGroupFiltered, ErrKeyConfigurationNotAllowed
	}

	return isGroupFiltered, nil
}

func (u *user) GetBusinessUserInfo(ctx context.Context) (BusinessUserInfo, error) {
	err := ensureBusinessUserOpsAllowed(ctx, []constants.InternalRole{
		constants.InternalTaskWorkflowApproversRole,
		constants.InternalTaskWorkflowExpirationRole,
		constants.InternalTenantProvisioningRole,
	})
	if errors.Is(err, errInternalBypass) {
		return BusinessUserInfo{}, nil
	}
	if err != nil {
		return BusinessUserInfo{}, err
	}

	businessUserData, err := cmkcontext.ExtractBusinessUserData(ctx)
	if err != nil {
		return BusinessUserInfo{}, err
	}

	groups, err := cmkcontext.ExtractBusinessUserDataGroups(ctx)
	if err != nil {
		return BusinessUserInfo{}, err
	}

	role, err := u.GetRoleFromIAM(ctx, groups)
	if err != nil {
		return BusinessUserInfo{}, err
	}

	return BusinessUserInfo{
		Identifier: businessUserData.Identifier,
		Email:      businessUserData.Email,
		GivenName:  businessUserData.GivenName,
		FamilyName: businessUserData.FamilyName,
		Role:       string(role),
	}, nil
}

func (u *user) HasTenantAccess(ctx context.Context) (bool, error) {
	iamIdentifiers, err := cmkcontext.ExtractBusinessUserDataGroups(ctx)
	if err != nil {
		return false, ErrTenantNotAllowed
	}

	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	authCtx, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalBusinessAuthzRole)
	if err != nil {
		return false, err
	}

	count, err := u.repo.Count(
		authCtx, &model.Group{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).SetLimit(0),
	)
	if err != nil {
		return false, errs.Wrap(ErrCheckTenantHasIAMGroups, err)
	}

	return count > 0, nil
}

func (u *user) GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.BusinessRole, error) {
	ck := repo.NewCompositeKey().Where(repo.IAMIdField, iamIdentifiers)

	authCtx, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalBusinessAuthzRole)
	if err != nil {
		return "", err
	}

	var groups []model.Group

	err = u.repo.List(
		authCtx, &model.Group{}, &groups,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return "", errs.Wrap(ErrGetGroups, err)
	}

	if len(groups) == 0 {
		return "", nil
	}

	roleMap := map[constants.BusinessRole]bool{}
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

// roleBypassesGroupFilter returns true when the given role grants full visibility
// for the resource on read, meaning no group-level filtering is needed.
func (u *user) roleBypassesGroupFilter(role constants.BusinessRole, resource authz.APIResourceType) bool {
	// Tenant auditor has read-only access to all data
	if role == constants.TenantAuditorRole {
		return true
	}
	// Tenant admin has access to all groups
	if role == constants.TenantAdminRole && resource == authz.APIResourceTypeUserGroup {
		return true
	}
	// Key admin has access to all systems
	return role == constants.KeyAdminRole && resource == authz.APIResourceTypeSystem
}

func (u *user) isCreateKeyconfig(
	keyConfig *model.KeyConfiguration,
	action authz.APIAction,
	resource authz.APIResourceType,
) bool {
	return action == authz.APIActionCreate &&
		resource == authz.APIResourceTypeKeyConfiguration &&
		(keyConfig.AdminGroup != model.Group{})
}

// hasKeyConfigAccess checks if a specific key configuration is managed by the user
// IAM groups
func (u *user) hasKeyConfigAccess(
	ctx context.Context,
	keyConfig *model.KeyConfiguration,
	action authz.APIAction,
	resource authz.APIResourceType,
) (bool, error) {
	iamIdentifiers, err := cmkcontext.ExtractBusinessUserDataGroupsString(ctx)
	if err != nil {
		return false, err
	}

	role, err := u.GetRoleFromIAM(ctx, iamIdentifiers)
	if err != nil {
		return false, err
	}

	// Auditors have read-only access to all keyconfigs
	if role == constants.TenantAuditorRole && action == authz.APIActionRead {
		return true, nil
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

	authCtx, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalBusinessAuthzRole)
	if err != nil {
		return false, err
	}

	count, err := u.repo.Count(authCtx, &model.KeyConfiguration{}, query)
	if err != nil {
		return false, errs.Wrap(ErrCheckKeyConfigManagedByIAMGroups, err)
	}

	return count > 0, nil
}

func (u *user) sendUnauthorizedAccessAuditLog(
	ctx context.Context,
	resource authz.APIResourceType,
	action authz.APIAction,
) {
	err := u.cmkAuditor.SendCmkUnauthorizedRequestAuditLog(ctx, string(resource), string(action))
	if err != nil {
		log.Error(ctx, "Failed to send unauthorized access audit log", err)
	}

	log.Info(ctx, "Sent unauthorized access audit log")
}

// ensureBusinessUserOpsAllowed checks the context before entering code paths that
// require business user data (e.g. IAM group extraction). It returns:
//   - a bypass sentinel (errInternalBypass) if the context carries a permitted internal
//     role — callers should return immediately with a zero/nil result
//   - ErrInternalRoleNotSupported if the context carries any other internal role —
//     this operation is not valid for that role
//   - nil if the context is a normal business user context — fall through to business logic
var errInternalBypass = errors.New("internal bypass")

func ensureBusinessUserOpsAllowed(ctx context.Context, bypassRoles []constants.InternalRole) error {
	err := authz.CheckInternalUserRoles(ctx, bypassRoles)
	if err == nil {
		return errInternalBypass
	}

	if errors.Is(err, authz.ErrAuthzUnauthorized) {
		// An internal role is present but is not in the bypass list — this operation
		// is not supported for that role.
		return ErrInternalRoleNotSupported
	}

	// ErrExtractInternalUserData — no internal role in context, this is a business user.
	return nil
}
