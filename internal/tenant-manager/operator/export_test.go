package operator

import (
	"context"

	"github.com/openkcm/orbital"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/repo"
)

func (o *TenantOperator) HandleCreateTenant(ctx context.Context, req orbital.HandlerRequest) (
	orbital.HandlerResponse, error) {
	return o.handleCreateTenant(ctx, req)
}

func HandleBlockTenant(ctx context.Context, req orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return handleBlockTenant(ctx, req)
}

func HandleUnblockTenant(ctx context.Context, req orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return handleUnblockTenant(ctx, req)
}

func HandleTerminateTenant(ctx context.Context, req orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return handleTerminateTenant(ctx, req)
}

func HandleApplyTenantAuth(ctx context.Context, req orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return handleApplyTenantAuth(ctx, req)
}

func CheckTenantSchemaExistenceStatus(
	ctx context.Context,
	db *multitenancy.DB,
	schemaName string,
) (SchemaExistenceStatus, error) {
	return checkTenantSchemaExistenceStatus(ctx, db, schemaName)
}

func CheckTenantGroupsExistenceStatus(
	ctx context.Context,
	r repo.Repo,
	tenantID string,
) (GroupsExistenceStatus, error) {
	return checkTenantGroupsExistenceStatus(ctx, r, tenantID)
}

func IsSchemaExists(ctx context.Context, db *multitenancy.DB, schemaName string) (bool, error) {
	return schemaExists(ctx, db, schemaName)
}

func IsGroupExists(ctx context.Context, r repo.Repo, groupType, tenantID string) (bool, error) {
	return groupExists(ctx, r, groupType, tenantID)
}
