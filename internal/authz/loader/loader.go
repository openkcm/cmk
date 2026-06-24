package authz_loader

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrLoadAuthzAllowList = errors.New("failed to load authz allow list for tenantID")
	ErrTenantNotExist     = errors.New("tenantID does not exist")
	ErrEmptyTenantID      = errors.New("tenantID cannot be empty")
)

type AuthzLoader[
	Resource authz.APIResourceType | authz.RepoResourceType,
	Action authz.APIAction | authz.RepoAction,
] struct {
	repo         repo.Repo
	TenantIDs    map[authz.TenantID]struct{} // Used as a cache for reload/adding new tenants
	AuthzHandler *authz.Handler[Resource, Action]
	mu           *sync.Mutex // protects AuthzHandler.Entities and AuthorizationData
	Auditor      *auditor.Auditor
}

func NewAuthzLoader[
	ResourceType authz.APIResourceType | authz.RepoResourceType,
	Action authz.APIAction | authz.RepoAction,
](
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
	internalRolePolicies authz.RolePolicies[constants.InternalRole, ResourceType, Action],
	businessRolePolicies authz.RolePolicies[constants.BusinessRole, ResourceType, Action],
	resourceTypeActions map[ResourceType][]Action,
) *AuthzLoader[ResourceType, Action] {
	audit := auditor.New(ctx, config)

	mu := sync.Mutex{}

	authzHandler, err := authz.NewAuthorizationHandler(
		audit,
		internalRolePolicies,
		businessRolePolicies,
		resourceTypeActions,
		&mu,
	)
	if err != nil {
		log.Error(ctx, "failed to create authorization handler", err)
		return nil
	}

	return &AuthzLoader[ResourceType, Action]{
		repo:         repo,
		TenantIDs:    make(map[authz.TenantID]struct{}),
		AuthzHandler: authzHandler,
		Auditor:      audit,
		mu:           &mu,
	}
}

func NewAPIAuthzLoader(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
) *AuthzLoader[authz.APIResourceType, authz.APIAction] {
	// No internal user access allowed to api
	APIInternalPolicies := make(authz.RolePolicies[constants.InternalRole, authz.APIResourceType, authz.APIAction])
	return NewAuthzLoader(ctx, repo, config, APIInternalPolicies, authz.APIPolicies, authz.APIResourceTypeActions)
}

func NewRepoAuthzLoader(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
) *AuthzLoader[authz.RepoResourceType, authz.RepoAction] {
	return NewAuthzLoader(
		ctx,
		repo,
		config,
		authz.RepoInternalPolicies,
		authz.RepoBusinessPolicies,
		authz.RepoResourceTypeActions,
	)
}

func (am *AuthzLoader[Resource, Action]) LoadTenantAllowedActions(ctx context.Context) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		// This could be internal user (for example). If we can't extract we assume not
		// relevant, otherwise will get tenant error on the assertion
		return nil
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	return am.loadTenantAllowedActions(ctx, tenantID)
}

func (am *AuthzLoader[Resource, Action]) ReloadTenantAllowedActions(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Take a copy of tenants IDs to update before resetting
	tenantIDs := am.TenantIDs
	am.ResetBusinessUserData()

	for tenantID := range tenantIDs {
		err := am.loadTenantAllowedActions(ctx, string(tenantID))
		if err != nil {
			return errs.Wrap(ErrLoadAuthzAllowList, err)
		}
	}

	return nil
}

func (am *AuthzLoader[Resource, Action]) ResetBusinessUserData() {
	am.TenantIDs = make(map[authz.TenantID]struct{})
	am.AuthzHandler.ResetBusinessUserData()
}

// StartAuthzDataRefresh starts a background goroutine that refreshes the authorization data periodically
func (am *AuthzLoader[Resource, Action]) StartAuthzDataRefresh(
	ctx context.Context, interval time.Duration,
) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info(ctx, "Stopping periodic authorization data refresh")
				return
			case <-ticker.C:
				log.Debug(ctx, "Starting periodic authorization data refresh")

				err := am.ReloadTenantAllowedActions(ctx)
				if err != nil {
					log.Error(ctx, "Failed to refresh authorization data", err)
				} else {
					log.Debug(ctx, "Successfully refreshed authorization data")
				}
			}
		}
	}()
}

// Loads the authorization allow list for a specific tenant, locking is done by caller.
// It retrieves all groups from the repository, maps them to roles, and updates the AuthzHandler.
// If the tenantID is empty or invalid, it returns an error.
// If the tenantID already exists in the AuthzHandler, it does nothing.
// If there are no groups, it does not update the AuthzHandler.
// If there are groups, it creates entities for each role and updates the AuthzHandler's
// AuthorizationData with the new entities.
//
// nolintfunlen
func (am *AuthzLoader[Resource, Action]) loadTenantAllowedActions(
	ctx context.Context,
	tenantID string,
) error {
	ctx = slogctx.With(ctx, slog.String("tenantId", tenantID))

	// Validate tenantID
	if tenantID == "" {
		return ErrEmptyTenantID
	}

	if !am.isTenantKnown(ctx, tenantID) {
		return ErrTenantNotExist
	}

	if _, exists := am.TenantIDs[authz.TenantID(tenantID)]; exists {
		log.Debug(ctx, "tenantId already exists in AuthzHandler, skipping load")
		return nil
	}

	groups, err := am.getGroups(ctx)
	if err != nil {
		return err
	}

	tenantRoleGroupsMap := make(map[constants.BusinessRole]*authz.BusinessUser)
	for _, group := range groups {
		role := group.Role
		user, exists := tenantRoleGroupsMap[role]

		if exists {
			user.Groups = append(user.Groups, group.IAMIdentifier)
		} else {
			tenantRoleGroupsMap[role] = &authz.BusinessUser{
				TenantID: authz.TenantID(tenantID),
				Groups:   []string{group.IAMIdentifier},
			}
		}
	}

	if len(tenantRoleGroupsMap) > 0 {
		err = am.AuthzHandler.UpdateBusinessUserData(tenantRoleGroupsMap)
		if err != nil {
			return errs.Wrap(ErrLoadAuthzAllowList, err)
		}
	}

	// Add tenant ID to the list of tenant IDs in case it is not already present
	if _, exists := am.TenantIDs[authz.TenantID(tenantID)]; !exists {
		am.TenantIDs[authz.TenantID(tenantID)] = struct{}{}
	}

	return nil
}

func (am *AuthzLoader[Resource, Action]) getGroups(ctx context.Context) ([]model.Group, error) {
	var groups []model.Group

	err := am.repo.List(ctx, &model.Group{}, &groups, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrLoadAuthzAllowList, err)
	}

	return groups, nil
}

func (am *AuthzLoader[Resource, Action]) isTenantKnown(ctx context.Context, tenantID string) bool {
	var tenant model.Tenant

	found, err := am.repo.First(
		ctx, &tenant,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IDField, tenantID),
		)),
	)
	if err != nil || !found {
		return false
	}

	return true
}
