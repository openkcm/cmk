package operator_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/operator"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	existingTenantName    = "existing_tenant"
	nonExistingTenantName = "non_existing_tenant"
	tenantWithGroupsName  = "tenant_with_groups"
)

func SetupProbeTest(t *testing.T) (*manager.GroupManager, *manager.TenantManager,
	*multitenancy.DB, repo.Repo) {
	t.Helper()

	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&model.Tenant{}, &model.Group{}},
	})

	dbRepository := sql.NewRepository(db)

	ctlg, err := catalog.New(t.Context(), config.Config{
		Plugins: testutils.SetupMockPlugins(testutils.IdentityPlugin),
	})
	assert.NoError(t, err, "Failed to create catalog")

	gm := manager.NewGroupManager(dbRepository, ctlg)
	tm := manager.NewTenantManager(dbRepository)

	return gm, tm, db, dbRepository
}

func TestTenantProbe_Check(t *testing.T) {
	ctx := t.Context()
	gm, tm, multitenancyDB, _ := SetupProbeTest(t)

	tenantID1 := uuid.NewString()
	tenantID2 := uuid.NewString()
	tenantID3 := uuid.NewString()
	tenantID4 := uuid.NewString()

	existingTenant := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = existingTenantName
		l.DomainURL = "existing_tenant.example.com"
		l.ID = tenantID1
	})
	nonExistingTenant := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = nonExistingTenantName
		l.DomainURL = "non_existing_tenant.example.com"
		l.ID = tenantID2
	})
	tenantWithGroups := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = tenantWithGroupsName
		l.DomainURL = "tenant_with_groups.example.com"
		l.ID = tenantID3
	})
	tenantWithNilDB := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "nil_db_tenant"
		l.DomainURL = "nil_db.example.com"
		l.ID = tenantID4
	})

	err := tm.CreateTenant(ctx, existingTenant)
	require.NoError(t, err, "Failed to create tenant schema for test")

	err = tm.CreateTenant(ctx, tenantWithGroups)
	require.NoError(t, err, "Failed to create tenant schema for test")

	groupCtx := cmkcontext.CreateTenantContext(ctx, tenantWithGroups.ID)
	err = gm.CreateDefaultGroups(groupCtx)
	require.NoError(t, err, "Failed to create tenant groups for test")

	tests := []struct {
		name                  string
		tenant                *model.Tenant
		db                    *multitenancy.DB
		schemaExistenceStatus operator.SchemaExistenceStatus
		groupsExistenceStatus operator.GroupsExistenceStatus
		wantErr               bool
		errContains           string
	}{
		{
			name:                  "tenant exists, groups does not exist",
			tenant:                existingTenant,
			db:                    multitenancyDB,
			schemaExistenceStatus: operator.SchemaExists,
			groupsExistenceStatus: operator.GroupsNotFound,
			wantErr:               false,
		},
		{
			name:                  "tenant exists, groups exist",
			tenant:                tenantWithGroups,
			db:                    multitenancyDB,
			schemaExistenceStatus: operator.SchemaExists,
			groupsExistenceStatus: operator.GroupsExist,
			wantErr:               false,
		},
		{
			name:                  "tenant does not exist",
			tenant:                nonExistingTenant,
			db:                    multitenancyDB,
			schemaExistenceStatus: operator.SchemaNotFound,
			groupsExistenceStatus: operator.GroupsNotFound,
			wantErr:               false,
		},
		{
			name:                  "nil database",
			tenant:                tenantWithNilDB,
			db:                    nil,
			schemaExistenceStatus: operator.SchemaCheckFailed,
			groupsExistenceStatus: operator.GroupsNotFound,
			wantErr:               true,
			errContains:           "database connection not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := &operator.TenantProbe{
				MultitenancyDB: tt.db,
				Repo:           sql.NewRepository(tt.db),
			}

			probeResult, err := probe.Check(ctx, tt.tenant)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error but got none")

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}

			assert.Equal(t, tt.schemaExistenceStatus, probeResult.SchemaStatus)
			assert.Equal(t, tt.groupsExistenceStatus, probeResult.GroupsStatus)
		})
	}
}

func TestCheckTenantSchemaExistenceStatus(t *testing.T) {
	ctx := t.Context()
	_, tm, multitenancyDB, _ := SetupProbeTest(t)

	tenantID := uuid.NewString()
	existingTenant := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "existing_schema"
		l.DomainURL = "existing.example.com"
		l.ID = tenantID
	})

	err := tm.CreateTenant(ctx, existingTenant)
	require.NoError(t, err, "Failed to create tenant schema for test")

	tests := []struct {
		name       string
		db         *multitenancy.DB
		schemaName string
		want       operator.SchemaExistenceStatus
		wantErr    bool
	}{
		{
			name:       "schema exists",
			db:         multitenancyDB,
			schemaName: "existing_schema",
			want:       operator.SchemaExists,
			wantErr:    false,
		},
		{
			name:       "schema does not exist",
			db:         multitenancyDB,
			schemaName: "non_existing_schema",
			want:       operator.SchemaNotFound,
			wantErr:    false,
		},
		{
			name:       "nil database",
			db:         nil,
			schemaName: "any_schema",
			want:       operator.SchemaCheckFailed,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := operator.CheckTenantSchemaExistenceStatus(ctx, tt.db, tt.schemaName)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, status)
		})
	}
}

func TestCheckTenantGroupsExistenceStatus(t *testing.T) {
	ctx := t.Context()
	gm, tm, _, repository := SetupProbeTest(t)
	tenantID1 := uuid.NewString()
	tenantID2 := uuid.NewString()
	tenantID3 := uuid.NewString()

	tenantWithBothGroups := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "tenant_with_both_groups"
		l.DomainURL = "both_groups.example.com"
		l.ID = tenantID1
	})

	tenantWithNoGroups := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "tenant_with_no_groups"
		l.DomainURL = "no_groups.example.com"
		l.ID = tenantID2
	})

	err := tm.CreateTenant(ctx, tenantWithBothGroups)
	require.NoError(t, err, "Failed to create tenant schema for test")

	err = tm.CreateTenant(ctx, tenantWithNoGroups)
	require.NoError(t, err, "Failed to create tenant schema for test")

	groupCtx := cmkcontext.CreateTenantContext(ctx, tenantWithBothGroups.ID)
	err = gm.CreateDefaultGroups(groupCtx)
	require.NoError(t, err, "Failed to create tenant groups for test")

	tests := []struct {
		name     string
		tenantID string
		want     operator.GroupsExistenceStatus
		wantErr  bool
	}{
		{
			name:     "both groups exist",
			tenantID: tenantID1,
			want:     operator.GroupsExist,
			wantErr:  false,
		},
		{
			name:     "groups do not exist",
			tenantID: tenantID2,
			want:     operator.GroupsNotFound,
			wantErr:  false,
		},
		{
			name:     "non-existing tenant",
			tenantID: tenantID3,
			want:     operator.GroupsCheckFailed,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := operator.CheckTenantGroupsExistenceStatus(ctx, repository, tt.tenantID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, status)
		})
	}
}

func TestSchemaExists(t *testing.T) {
	ctx := t.Context()
	_, tm, multitenancyDB, _ := SetupProbeTest(t)

	tenantID := uuid.NewString()
	tenant := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "test_schema"
		l.DomainURL = "test.example.com"
		l.ID = tenantID
	})

	err := tm.CreateTenant(ctx, tenant)
	require.NoError(t, err, "Failed to create tenant schema for test")

	tests := []struct {
		name        string
		db          *multitenancy.DB
		schemaName  string
		want        bool
		wantErr     bool
		errContains string
	}{
		{
			name:       "schema exists",
			db:         multitenancyDB,
			schemaName: "test_schema",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "schema does not exist",
			db:         multitenancyDB,
			schemaName: "non_existing_schema",
			want:       false,
			wantErr:    false,
		},
		{
			name:        "nil database",
			db:          nil,
			schemaName:  "any_schema",
			want:        false,
			wantErr:     true,
			errContains: "database connection not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := operator.IsSchemaExists(ctx, tt.db, tt.schemaName)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, exists)
		})
	}
}

func TestGroupExists(t *testing.T) {
	ctx := t.Context()
	gm, tm, _, repository := SetupProbeTest(t)

	tenantID1 := uuid.NewString()
	tenantID2 := uuid.NewString()

	tenantWithGroups := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "tenant_with_groups_for_group_test"
		l.DomainURL = "group_test.example.com"
		l.ID = tenantID1
	})

	tenantWithoutGroups := testutils.NewTenant(func(l *model.Tenant) {
		l.SchemaName = "tenant_without_groups_for_group_test"
		l.DomainURL = "no_group_test.example.com"
		l.ID = tenantID2
	})

	err := tm.CreateTenant(ctx, tenantWithGroups)
	require.NoError(t, err, "Failed to create tenant schema for test")

	err = tm.CreateTenant(ctx, tenantWithoutGroups)
	require.NoError(t, err, "Failed to create tenant schema for test")

	groupCtx := cmkcontext.CreateTenantContext(ctx, tenantWithGroups.ID)
	err = gm.CreateDefaultGroups(groupCtx)
	require.NoError(t, err, "Failed to create tenant groups for test")

	tests := []struct {
		name      string
		groupType string
		tenantID  string
		want      bool
		wantErr   bool
	}{
		{
			name:      "admin group exists",
			groupType: constants.TenantAdminGroup,
			tenantID:  tenantID1,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "auditor group exists",
			groupType: constants.TenantAuditorGroup,
			tenantID:  tenantID1,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "admin group does not exist",
			groupType: constants.TenantAdminGroup,
			tenantID:  tenantID2,
			want:      false,
			wantErr:   false,
		},
		{
			name:      "auditor group does not exist",
			groupType: constants.TenantAuditorGroup,
			tenantID:  tenantID2,
			want:      false,
			wantErr:   false,
		},
		{
			name:      "non-existing tenant",
			groupType: constants.TenantAdminGroup,
			tenantID:  "non-existing-tenant-id",
			want:      false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := operator.IsGroupExists(ctx, repository, tt.groupType, tt.tenantID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, exists)
		})
	}
}
