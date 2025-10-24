package manager

import (
	"context"
	"sync"

	"github.com/openkcm/cmk-core/internal/authz"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

type AuthzManager struct {
	repo         repo.Repo
	AuthzHandler *authz.Handler
	mu           sync.Mutex // protects AuthzHandler.Entities and AuthorizationData
}

func NewAuthzManager(
	ctx context.Context,
	repo repo.Repo,
) *AuthzManager {
	// start with an empty list of entities
	entities := &[]authz.Entity{}

	authzHandler, err := authz.NewAuthorizationHandler(entities)
	if err != nil {
		log.Error(ctx, "failed to create authorization handler", err)
		return nil
	}

	return &AuthzManager{
		repo:         repo,
		AuthzHandler: authzHandler,
	}
}

func (am *AuthzManager) LoadAllowList(ctx context.Context, tenantID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	return am.loadAllowListInternal(ctx, tenantID)
}

func (am *AuthzManager) ReloadAllowList(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.AuthzHandler.Entities = []authz.Entity{}
	am.AuthzHandler.AuthorizationData = authz.AllowList{
		AuthzKeys: make(map[authz.AuthorizationKey]struct{}),
		TenantIDs: make(map[authz.TenantID]struct{}),
	}

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return errs.Wrap(ErrLoadAuthzAllowList, err)
	}

	err = am.loadAllowListInternal(ctx, tenantID)
	if err != nil {
		return errs.Wrap(ErrLoadAuthzAllowList, err)
	}

	return nil
}

// Loads the authorization allow list for a specific tenant, locking is done by caller.
// It retrieves all groups from the repository, maps them to roles, and updates the AuthzHandler.
// If the tenantID is empty or invalid, it returns an error.
// If the tenantID already exists in the AuthzHandler, it does nothing.
// If there are no groups, it does not update the AuthzHandler.
// If there are groups, it creates entities for each role and updates the AuthzHandler's
// AuthorizationData with the new entities.
func (am *AuthzManager) loadAllowListInternal(ctx context.Context, tenantID string) error {
	// Validate tenantID
	if tenantID == "" {
		return errs.Wrap(ErrTenantNotExist, ErrEmptyTenantID)
	}

	if !isTenantKnown(ctx, am.repo, tenantID) {
		return errs.Wrap(ErrTenantNotExist, ErrTenantNotExist)
	}

	if am.AuthzHandler.AuthorizationData.ContainsTenant(authz.TenantID(tenantID)) {
		return nil
	}

	groups, err := listGroups(ctx, am.repo)
	if err != nil {
		return err
	}

	roleToEntity := make(map[constants.Role]*authz.Entity)
	for _, group := range groups {
		role := group.Role
		if entity, exists := roleToEntity[role]; exists {
			entity.UserGroups = append(entity.UserGroups, authz.UserGroup(group.Name))
		} else {
			roleToEntity[role] = &authz.Entity{
				TenantID:   authz.TenantID(tenantID),
				Role:       role,
				UserGroups: []authz.UserGroup{authz.UserGroup(group.Name)},
			}
		}
	}

	entities := make([]authz.Entity, 0, len(roleToEntity))
	for _, entity := range roleToEntity {
		entities = append(entities, *entity)
	}

	if len(entities) > 0 {
		am.AuthzHandler.Entities = append(am.AuthzHandler.Entities, entities...)

		authzData, err := authz.NewAuthorizationData(am.AuthzHandler.Entities)
		if err != nil {
			return errs.Wrap(ErrLoadAuthzAllowList, err)
		}

		am.AuthzHandler.AuthorizationData = *authzData
	}

	return nil
}

func listGroups(ctx context.Context, amrepo repo.Repo) ([]model.Group, error) {
	var groups []model.Group

	_, err := amrepo.List(
		ctx, &model.Group{}, &groups, *repo.NewQuery(),
	)
	if err != nil {
		return nil, errs.Wrap(ErrLoadAuthzAllowList, err)
	}

	return groups, nil
}

func isTenantKnown(ctx context.Context, amrepo repo.Repo, tenantID string) bool {
	var tenant model.Tenant

	found, err := amrepo.First(
		ctx, &tenant,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IDField, tenantID))),
	)
	if err != nil || !found {
		return false
	}

	return true
}
