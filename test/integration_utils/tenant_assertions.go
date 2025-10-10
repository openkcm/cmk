package integrationutils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TableInfo struct {
	Schema string `gorm:"column:table_schema"`
	Name   string `gorm:"column:table_name"`
}

func (TableInfo) TableName() string {
	return "information_schema.tables"
}

func NamespaceExists(database *multitenancy.DB, tenantName string) (bool, error) {
	var exists bool

	err := database.
		Table("pg_catalog.pg_namespace").
		Select("count(1) > 0").
		Where("nspname = ?", tenantName).
		Scan(&exists).Error

	return exists, err
}

func TenantSchemaExists(database *multitenancy.DB, tenantName string) (bool, error) {
	var count int64

	err := database.
		Model(&TableInfo{}).
		Where("table_schema = ?", tenantName).
		Count(&count).Error

	return count > int64(0), err
}

func TableInTenantSchemaExist(database *multitenancy.DB, tenantName string,
	tableName string) (bool, error) {
	var tables []TableInfo

	err := database.
		Table("information_schema.tables").
		Where("table_schema = ? AND table_name = ?", tenantName, tableName).
		Order("table_schema, table_name").
		Find(&tables).Error

	return len(tables) == 1, err
}

func TenantExistsInPublicSchema(database *multitenancy.DB, tenantName string) (bool, error) {
	var count int64

	err := database.
		Model(&model.Tenant{}).
		Where("schema_name = ?", tenantName).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func tenantUserGroupsExists(
	ctx context.Context,
	database *multitenancy.DB,
	tenantID string,
	groupType string,
) (bool, error) {
	r := sql.NewRepository(database)

	groupctx := cmkcontext.CreateTenantContext(ctx, tenantID)

	exists, err := r.First(groupctx, &model.Group{}, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.Name, groupType))))

	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return false, err
	}

	if errors.Is(err, repo.ErrNotFound) {
		return false, nil
	}

	return exists, nil
}

func GroupsExists(ctx context.Context, t require.TestingT, tenantID string, multitenancydb *multitenancy.DB) {
	groups := []string{constants.TenantAdminGroup, constants.TenantAuditorGroup}
	for _, group := range groups {
		exists, err := tenantUserGroupsExists(ctx, multitenancydb, tenantID, group)
		assert.NoError(t, err, "Failed to check if group exists")
		assert.True(t, exists, "Group %s should exist", group)
	}
}

func TenantExists(t require.TestingT, multitenancydb *multitenancy.DB, schemaName, tableName string) {
	exists, err := TenantSchemaExists(multitenancydb, schemaName)
	assert.True(t, exists, "Schema %s should exist", schemaName)
	assert.NoError(t, err)

	exists, err = TenantExistsInPublicSchema(multitenancydb, schemaName)
	assert.True(t, exists, "Tenant %s should exist in public schema", schemaName)
	assert.NoError(t, err)

	exists, err = NamespaceExists(multitenancydb, schemaName)
	assert.True(t, exists, "Tenant %s namespace should exist", schemaName)
	assert.NoError(t, err)

	exists, err = TableInTenantSchemaExist(multitenancydb, schemaName, tableName)
	assert.True(t, exists, "Table %s should exist in tenant %s schema", tableName, schemaName)
	assert.NoError(t, err)
}

func CheckRegion(ctx context.Context, t *testing.T, database *multitenancy.DB, tenantID string, expectedRegion string) {
	t.Helper()

	r := sql.NewRepository(database)

	tenant := model.Tenant{}

	_, err := r.First(ctx, &tenant, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IDField, tenantID))))
	require.NoError(t, err, "Failed to get tenant %s", tenantID)
	assert.Equal(t, expectedRegion, tenant.Region, "Tenant %s region should be %s", tenantID, expectedRegion)
}
