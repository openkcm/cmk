package authzmodel

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
)

var (
	ErrLoadAuthzAllowList = errors.New("failed to load authz allow list for tenantID")
	ErrTenantNotExist     = errors.New("tenantID does not exist")
	ErrEmptyTenantID      = errors.New("tenantID cannot be empty")
)

type Engine struct {
	repo         repo.Repo
	AuthzHandler *authz.Handler
	mu           sync.Mutex // protects AuthzHandler.Entities and AuthorizationData
	Auditor      *auditor.Auditor
}

func NewEngine(
	ctx context.Context,
	repo repo.Repo,
	config *config.Config,
) *Engine {
	// start with an empty list of entities
	entities := &[]authz.Entity{}

	audit := auditor.New(ctx, config)

	authzHandler, err := authz.NewAuthorizationHandler(entities, audit)
	if err != nil {
		log.Error(ctx, "failed to create authorization handler", err)
		return nil
	}

	return &Engine{
		repo:         repo,
		AuthzHandler: authzHandler,
		Auditor:      audit,
	}
}

func (am *Engine) LoadAllowList(ctx context.Context, tenantID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	return am.loadAllowListInternal(ctx, tenantID)
}

func (am *Engine) ReloadAllowList(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Collect all tenants which were previously loaded
	tenantList := make([]authz.TenantID, 0, len(am.AuthzHandler.Entities))
	for _, entity := range am.AuthzHandler.Entities {
		tenantList = append(tenantList, entity.TenantID)
	}

	am.AuthzHandler.Entities = []authz.Entity{}
	am.AuthzHandler.AuthorizationData = authz.AllowList{
		AuthzKeys: make(map[authz.AuthorizationKey]struct{}),
		TenantIDs: make(map[authz.TenantID]struct{}),
	}

	for _, tenantID := range tenantList {
		err := am.loadAllowListInternal(ctx, string(tenantID))
		if err != nil {
			return errs.Wrap(ErrLoadAuthzAllowList, err)
		}
	}

	return nil
}

// StartAuthzDataRefresh starts a background goroutine that refreshes the authorization data periodically
func (am *Engine) StartAuthzDataRefresh(ctx context.Context, interval time.Duration) {
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
func (am *Engine) loadAllowListInternal(ctx context.Context, tenantID string) error {
	// Validate tenantID
	if tenantID == "" {
		return errs.Wrap(ErrTenantNotExist, ErrEmptyTenantID)
	}

	if !isTenantKnown(ctx, am.repo, tenantID) {
		return errs.Wrap(ErrTenantNotExist, ErrTenantNotExist)
	}

	if am.AuthzHandler.AuthorizationData.ContainsTenant(authz.TenantID(tenantID)) {
		slog.Debug(
			"tenantID", "tenantID", tenantID, "message", "tenantID already exists in AuthzHandler, skipping load",
		)

		return nil
	}

	groups, err := listGroups(ctx, am.repo)
	slog.Debug("tenantID", "tenantID", tenantID, "groups", len(groups), "err", err)

	if err != nil {
		return err
	}

	roleToEntity := make(map[constants.Role]*authz.Entity)

	for _, group := range groups {
		role := group.Role
		if entity, exists := roleToEntity[role]; exists {
			entity.UserGroups = append(entity.UserGroups, group.IAMIdentifier)
		} else {
			roleToEntity[role] = &authz.Entity{
				TenantID:   authz.TenantID(tenantID),
				Role:       role,
				UserGroups: []string{group.IAMIdentifier},
			}
		}
	}

	slog.Debug("tenantID", "tenantID", tenantID, "roleToEntity", len(roleToEntity))

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

		slog.Debug("tenantID", "tenantID", tenantID, "authzData", authzData)
		am.AuthzHandler.AuthorizationData = *authzData
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
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IDField, tenantID))),
	)
	if err != nil || !found {
		return false
	}

	return true
}
