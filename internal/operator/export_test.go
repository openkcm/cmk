package operator

import (
	"context"

	"github.com/openkcm/orbital"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/repo"
)

func (o *TenantOperator) HandleCreateTenant(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) {
	o.handleCreateTenant(ctx, req, resp)
}

func (o *TenantOperator) HandleApplyTenantAuth(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) {
	o.handleApplyTenantAuth(ctx, req, resp)
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

func ParseCommaSeparatedValues(input string) []string {
	return parseCommaSeparatedValues(input)
}

func ExtractOIDCConfig(properties map[string]string) (OIDCConfig, error) {
	return extractOIDCConfig(properties)
}
