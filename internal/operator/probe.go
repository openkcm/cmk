package operator

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

const (
	probeTimeoutSeconds = 5
)

type SchemaExistenceStatus int

type GroupsExistenceStatus int

const (
	// SchemaNotFound indicates that the schema doesn't exist yet or is still being created
	SchemaNotFound SchemaExistenceStatus = iota + 1
	// SchemaExists indicates that the schema exists in the database
	SchemaExists
	// SchemaCheckFailed indicates that there was an error checking for schema existence
	SchemaCheckFailed
)

const (
	// GroupsNotFound indicates that one or more required groups don't exist yet or are still being created
	GroupsNotFound GroupsExistenceStatus = iota + 1
	// GroupsExist indicates that all required groups exist
	GroupsExist
	// GroupsCheckFailed indicates that there was an error checking for group existence
	GroupsCheckFailed
)

// TenantProbeResult holds the statuses of the tenant schema and group existence checks.
type TenantProbeResult struct {
	SchemaStatus SchemaExistenceStatus
	GroupsStatus GroupsExistenceStatus
}

// TenantProbe checks for the existence of tenant resources.
type TenantProbe struct {
	MultitenancyDB *multitenancy.DB
	Repo           repo.Repo
}

// Check verifies the existence of tenant schema and groups for the provided tenant.
// It returns a TenantProbeResult with the current status of schema and group existence,
// along with any error that occurred during the check.
func (p *TenantProbe) Check(ctx context.Context, tenant *model.Tenant) (TenantProbeResult, error) {
	schemaName := tenant.SchemaName
	tenantID := tenant.ID

	result := TenantProbeResult{
		SchemaStatus: SchemaNotFound,
		GroupsStatus: GroupsNotFound,
	}

	schemaCtx, cancelSchema := context.WithTimeout(ctx, probeTimeoutSeconds*time.Second)
	defer cancelSchema()

	schemaStatus, err := checkTenantSchemaExistenceStatus(schemaCtx, p.MultitenancyDB, schemaName)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn(ctx, "Timeout reached during schema existence check.")
			return result, err
		}

		if errors.Is(err, ErrSchemaNotFound) {
			log.Warn(ctx, "Schema not found, skipping group check.")
			return result, nil
		}

		result.SchemaStatus = SchemaCheckFailed

		return result, err
	}

	result.SchemaStatus = schemaStatus

	// If schema doesn't exist, don't check groups
	if schemaStatus == SchemaNotFound {
		return result, nil
	}

	groupCtx, cancelGroup := context.WithTimeout(ctx, probeTimeoutSeconds*time.Second)
	defer cancelGroup()

	// Check group existence with timeout context
	groupsStatus, err := checkTenantGroupsExistenceStatus(groupCtx, p.Repo, tenantID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn(ctx, "Timeout reached during group existence check.")
			return result, err
		}

		result.GroupsStatus = GroupsCheckFailed

		return result, err
	}

	result.GroupsStatus = groupsStatus

	return result, nil
}

// checkTenantSchemaExistenceStatus determines the current existence status of a tenant schema.
// It checks if the tenant schema exists in the database and returns the appropriate SchemaExistenceStatus.
//
// Returns:
//   - SchemaExists if the schema exists
//   - SchemaNotFound if the schema doesn't exist yet
//   - SchemaCheckFailed if there was an error during the check
//
// If the context is already canceled or timed out, it returns early with SchemaCheckFailed.
func checkTenantSchemaExistenceStatus(
	ctx context.Context,
	db *multitenancy.DB,
	schemaName string,
) (SchemaExistenceStatus, error) {
	exists, err := schemaExists(ctx, db, schemaName)
	if err != nil {
		return SchemaCheckFailed, err
	}

	if !exists {
		return SchemaNotFound, nil
	}

	return SchemaExists, nil
}

// checkTenantGroupsExistenceStatus determines the current existence status of tenant groups.
// It checks if both the admin and auditor groups exist for the given tenant.
//
// Returns:
//   - GroupsExist if both admin and auditor groups exist
//   - GroupsNotFound if either group doesn't exist yet
//   - GroupsCheckFailed if there was an error during the check
func checkTenantGroupsExistenceStatus(
	ctx context.Context,
	r repo.Repo,
	tenantID string,
) (GroupsExistenceStatus, error) {
	adminExists, err := groupExists(ctx, r, constants.TenantAdminGroup, tenantID)
	if err != nil {
		return GroupsCheckFailed, err
	}

	auditorExists, err := groupExists(ctx, r, constants.TenantAuditorGroup, tenantID)
	if err != nil {
		return GroupsCheckFailed, err
	}

	if !adminExists || !auditorExists {
		return GroupsNotFound, nil
	}

	return GroupsExist, nil
}

// schemaExists checks if the tenant schema exists in the database.
// This is a low-level function that directly interacts with the database.
//
// Returns:
//   - true if the schema exists
//   - false if the schema doesn't exist
//   - an error if the database is nil or there was an error executing the query
func schemaExists(ctx context.Context, db *multitenancy.DB, schemaName string) (bool, error) {
	if db == nil {
		return false, errs.Wrap(ErrCheckingTenantExistence, ErrUninitializedDatabase)
	}

	err := db.WithContext(ctx).Where("schema_name = ?", schemaName).First(&model.Tenant{}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil // Schema does not exist
		}

		return false, errs.Wrap(ErrCheckingTenantExistence, err)
	}

	return true, nil
}

// groupExists checks if a specific tenant group exists in the tenant schema.
// This is a low-level function that directly interacts with the database.
//
// Returns:
//   - true if the group exists
//   - false if the group doesn't exist
//   - an error if there was an error executing the query, excluding not found errors
func groupExists(ctx context.Context, r repo.Repo, groupType, tenantID string) (bool, error) {
	tCtx := cmkcontext.CreateTenantContext(ctx, tenantID)

	group := model.Group{}

	exists, err := r.First(tCtx, &group, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.Name, groupType))))
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return false, errs.Wrap(ErrGroupNotFound, err)
	}

	return exists, nil
}
