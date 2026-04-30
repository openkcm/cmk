package authz_loader

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

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

type AuthzLoader[TResourceTypeName, TAction comparable] struct {
	repo         repo.Repo
	TenantIDs    map[authz.TenantID]struct{}
	AuthzHandler *authz.Handler[TResourceTypeName, TAction]
	mu           sync.Mutex // protects AuthzHandler.Entities and AuthorizationData
	Auditor      *auditor.Auditor
}

func NewAuthzLoader[TResourceTypeName, TAction comparable](
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
	internalRolePolicies map[constants.InternalRole][]authz.BasePolicy[constants.InternalRole,
		TResourceTypeName, TAction],
	businessRolePolicies map[constants.BusinessRole][]authz.BasePolicy[constants.BusinessRole,
		TResourceTypeName, TAction],
	resourceTypeActions map[TResourceTypeName][]TAction,
) *AuthzLoader[TResourceTypeName, TAction] {
	audit := auditor.New(ctx, config)

	authzHandler, err := authz.NewAuthorizationHandler(audit,
		internalRolePolicies, businessRolePolicies, resourceTypeActions)
	if err != nil {
		log.Error(ctx, "failed to create authorization handler", err)
		return nil
	}

	return &AuthzLoader[TResourceTypeName, TAction]{
		repo:         repo,
		TenantIDs:    make(map[authz.TenantID]struct{}),
		AuthzHandler: authzHandler,
		Auditor:      audit,
	}
}

func NewAPIAuthzLoader(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
) *AuthzLoader[authz.APIResourceTypeName, authz.APIAction] {
	// No internal user access allowed to api
	APIInternalPolicies := make(map[constants.InternalRole][]authz.BasePolicy[constants.InternalRole,
		authz.APIResourceTypeName, authz.APIAction])
	return NewAuthzLoader(ctx, repo, config, APIInternalPolicies, authz.APIBusinessPolicies, authz.APIResourceTypeActions)
}

func NewRepoAuthzLoader(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
) *AuthzLoader[authz.RepoResourceTypeName, authz.RepoAction] {
	return NewAuthzLoader(ctx, repo, config,
		authz.RepoInternalPolicies, authz.RepoBusinessPolicies, authz.RepoResourceTypeActions)
}

func (am *AuthzLoader[TResourceTypeName, TAction]) LoadAllowList(ctx context.Context) error {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		// This could be internal user (for example). If we can't extract we assume not
		// relevant, otherwise will get tenant error on the assertion
		return nil
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	return am.loadAllowListInternal(ctx, tenantID)
}

func (am *AuthzLoader[TResourceTypeName, TAction]) ReloadAllowList(
	ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Take a copy of tenants IDs to update before resetting
	tenantIDs := am.TenantIDs
	am.ResetBusinessUserData()

	for tenantID, _ := range tenantIDs {
		err := am.loadAllowListInternal(ctx, string(tenantID))
		if err != nil {
			return errs.Wrap(ErrLoadAuthzAllowList, err)
		}
	}

	return nil
}

func (am *AuthzLoader[TResourceTypeName, TAction]) ResetBusinessUserData() {
	am.TenantIDs = make(map[authz.TenantID]struct{})
	am.AuthzHandler.ResetBusinessUserData()
}

// StartAuthzDataRefresh starts a background goroutine that refreshes the authorization data periodically
func (am *AuthzLoader[TResourceTypeName, TAction]) StartAuthzDataRefresh(
	ctx context.Context, interval time.Duration) {
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

				err := am.ReloadAllowList(ctx)
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
func (am *AuthzLoader[TResourceTypeName, TAction]) loadAllowListInternal(
	ctx context.Context, tenantID string) error {
	// Validate tenantID
	if tenantID == "" {
		return errs.Wrap(ErrTenantNotExist, ErrEmptyTenantID)
	}

	if !isTenantKnown(ctx, am.repo, tenantID) {
		return errs.Wrap(ErrTenantNotExist, ErrTenantNotExist)
	}

	if _, exists := am.TenantIDs[authz.TenantID(tenantID)]; exists {
		slog.Debug(
			"tenantId", "tenantId", tenantID, "message", "tenantId already exists in AuthzHandler, skipping load",
		)

		return nil
	}

	groups, err := listGroups(ctx, am.repo)
	slog.Debug("tenantId", "tenantId", tenantID, "groups", len(groups), "err", err)

	if err != nil {
		return err
	}

	roleToEntity := make(map[constants.BusinessRole]*authz.Entity[
		constants.BusinessRole, authz.BusinessUserEntity])

	for _, group := range groups {
		role := group.Role
		if entity, exists := roleToEntity[role]; exists {
			entity.User.Groups = append(entity.User.Groups, group.IAMIdentifier)
		} else {
			roleToEntity[role] = &authz.Entity[
				constants.BusinessRole, authz.BusinessUserEntity]{
				User: authz.BusinessUserEntity{
					TenantID: authz.TenantID(tenantID),
					Groups:   []string{group.IAMIdentifier},
				},
				Role: role,
			}
		}
	}

	slog.Debug("tenantId", "tenantId", tenantID, "roleToEntity", len(roleToEntity))

	entities := make([]authz.Entity[
		constants.BusinessRole, authz.BusinessUserEntity], 0, len(roleToEntity))
	for _, entity := range roleToEntity {
		entities = append(entities, *entity)
	}

	if len(entities) > 0 {
		err = am.AuthzHandler.UpdateBusinessUserData(entities)
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

func listGroups(ctx context.Context, amrepo repo.Repo) ([]model.Group, error) {
	var groups []model.Group

	err := amrepo.List(ctx, &model.Group{}, &groups, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrLoadAuthzAllowList, err)
	}

	return groups, nil
}

func isTenantKnown(ctx context.Context, amrepo repo.Repo, tenantID string) bool {
	var tenant model.Tenant

	found, err := amrepo.First(
		ctx, &tenant,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IDField, tenantID))),
	)
	if err != nil || !found {
		return false
	}

	return true
}
