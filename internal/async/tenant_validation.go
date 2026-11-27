package async

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// tenantValidationRepo is the process-wide repo used to confirm a payload's
// tenant exists. It is nil until SetTenantValidationRepo runs; a nil repo skips
// the existence check (the empty-ID guard still applies), so an unconfigured
// deployment keeps working.
var tenantValidationRepo atomic.Pointer[repoHolder]

type repoHolder struct{ repo repo.Repo }

// SetTenantValidationRepo installs the process-wide repo for fan-out tenant
// validation. Call once at startup.
func SetTenantValidationRepo(r repo.Repo) {
	tenantValidationRepo.Store(&repoHolder{repo: r})
}

func defaultTenantValidationRepo() repo.Repo {
	if h := tenantValidationRepo.Load(); h != nil {
		return h.repo
	}
	return nil
}

// validateTenantIDsExist rejects an empty tenant ID, and when r is non-nil
// rejects any ID that does not match a known tenant. Tenant IDs are free-form
// strings, so the check is existence, not a format check.
func validateTenantIDsExist(ctx context.Context, r repo.Repo, tenantIDs []string) error {
	if slices.Contains(tenantIDs, "") {
		return ErrEmptyTenantID
	}

	if r == nil {
		return nil
	}

	unique := slices.Compact(slices.Sorted(slices.Values(tenantIDs)))

	ck := repo.NewCompositeKey().Where(repo.IDField, unique)
	found, err := r.Count(ctx, &model.Tenant{}, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return fmt.Errorf("counting tenants for payload validation: %w", err)
	}

	if found != len(unique) {
		return ErrUnknownTenantID
	}

	return nil
}
