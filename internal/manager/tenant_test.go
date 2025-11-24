package manager_test

import (
	"strings"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var (
	IssuerURL = "http://issuer-url"
)

func SetupTenantManager(t *testing.T, opts ...testutils.TestDBConfigOpt) (*manager.TenantManager,
	*multitenancy.DB, []string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models: []driver.TenantTabler{testutils.TestModel{}, &model.Tenant{}, &model.Group{},
			&model.KeystoreConfiguration{}},
	}, opts...)

	dbRepository := sql.NewRepository(db)

	m := manager.NewTenantManager(dbRepository)

	return m, db, tenants
}

func TestTenantManager(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		Models:         []driver.TenantTabler{&testutils.TestModel{}},
	}, testutils.WithGenerateTenants(10))
	r := sql.NewRepository(db)
	m := manager.NewTenantManager(r)

	t.Run("Should get tenant info", func(t *testing.T) {
		tenant := tenants[5]
		tenantModel, err := m.GetTenant(testutils.CreateCtxWithTenant(tenant))
		assert.NoError(t, err)
		assert.Equal(t, tenant, tenantModel.ID)
	})
	t.Run("Should list tenants", func(t *testing.T) {
		tenantsModel, _, err := m.ListTenantInfo(t.Context(), nil, 0, 0)
		assert.NoError(t, err)

		for i := range tenantsModel {
			assert.Equal(t, tenants[i], tenantsModel[i].ID)
		}
	})
}

func TestGetTenantByID(t *testing.T) {
	m, _, tenants := SetupTenantManager(t, testutils.WithGenerateTenants(1))
	tenant := tenants[0]

	tests := []struct {
		name     string
		tenantID string
		wantErr  bool
	}{
		{
			name:     "should get tenant by ID",
			tenantID: tenant,
			wantErr:  false,
		},
		{
			name:     "should return error for non-existing tenant ID",
			tenantID: "non-existing-tenant",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			result, err := m.GetTenantByID(ctx, tt.tenantID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.tenantID, result.ID)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name          string
		schema        string
		expectedError error
	}{
		{
			name:   "valid schema",
			schema: "KMS_validschema",
		},
		{
			name:          "schema name too long",
			schema:        "KMS_" + strings.Repeat("a", 60), // 64+ characters
			expectedError: manager.ErrSchemaNameLength,
		},
		{
			name:          "schema name too short",
			schema:        "sc",
			expectedError: manager.ErrInvalidSchema,
		},
		{
			name:          "namespace validation fails forbidden prefix",
			schema:        "pg_invalid",
			expectedError: manager.ErrInvalidSchema,
		},
		{
			name:          "namespace validation fails regex check",
			schema:        "invalid@name",
			expectedError: manager.ErrInvalidSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateSchema(tt.schema)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
