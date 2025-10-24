package db_test

import (
	"strings"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/tenant-manager/db"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
	"github.com/openkcm/cmk/utils/base62"
)

var (
	errInvalidPattern = "Tenant name must match the following pattern"
	errInvalidPrefix  = "Tenant name must not start with 'pg_' as it is reserved for system schemas in PostgreSQL"
)

var (
	tenantschemaName1 = "KMS_A123"
	tenantdomainName1 = tenantschemaName1 + ".example.com"
)

type ExampleModel struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

func (ExampleModel) TableName() string {
	return "example_models"
}

func (ExampleModel) IsSharedModel() bool {
	return false
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name          string
		schema        string
		expectedError string
	}{
		{
			name:          "valid schema",
			schema:        "KMS_validschema",
			expectedError: "",
		},
		{
			name:          "schema name too long",
			schema:        "KMS_" + strings.Repeat("a", 60), // 64+ characters
			expectedError: db.ErrSchemaNameLength.Error(),
		},
		{
			name:          "schema name too short",
			schema:        "sc",
			expectedError: errInvalidPattern,
		},
		{
			name:          "namespace validation fails forbidden prefix",
			schema:        "pg_invalid",
			expectedError: errInvalidPrefix,
		},
		{
			name:          "namespace validation fails regex check",
			schema:        "invalid@name",
			expectedError: errInvalidPattern,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.Validate(tt.schema)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateTenantSchema(t *testing.T) {
	ctx := t.Context()

	database, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Tenant{},
			&ExampleModel{},
		},
		RequiresMultitenancyOrShared: true,
	})

	var forced *testutils.ErrorForced

	tenantID := uuid.NewString()
	tenantID2 := uuid.NewString()

	tests := []struct {
		name       string
		tenant     *model.Tenant
		forceError func()
		setup      func()
		cleanup    func()

		tenantName  string
		domainURL   string
		expectError bool
		errorType   error
	}{
		{
			name: "creates and migrates tenant when not existing",
			tenant: testutils.NewTenant(func(t *model.Tenant) {
				t.SchemaName = tenantschemaName1
				t.DomainURL = tenantdomainName1
			}),
			expectError: false,
		},
		{
			name: "migrates existing tenant if already exists",
			tenant: testutils.NewTenant(func(t *model.Tenant) {
				t.ID = tenantID
				t.SchemaName, _ = base62.EncodeSchemaNameBase62(t.ID)
				t.DomainURL, _ = base62.EncodeSchemaNameBase62(t.ID)
			}),
			setup: func() {
				err := db.CreateSchema(ctx, database, testutils.NewTenant(func(t *model.Tenant) {
					t.ID = tenantID
					t.SchemaName, _ = base62.EncodeSchemaNameBase62(t.ID)
					t.DomainURL, _ = base62.EncodeSchemaNameBase62(t.ID)
				}))
				require.NoError(t, err)
			},
			expectError: true,
			errorType:   db.ErrOnboardingInProgress,
		},
		{
			name: "error when creating tenant, should not create tenant",
			tenant: testutils.NewTenant(func(t *model.Tenant) {
				t.SchemaName, _ = base62.EncodeSchemaNameBase62(tenantID2)
				t.DomainURL, _ = base62.EncodeSchemaNameBase62(tenantID2)
			}),
			forceError: func() {
				forced = testutils.NewDBErrorForced(database, gorm.ErrInvalidData)
				forced.WithCreate().Register()
			},
			cleanup: func() {
				forced.Unregister()
			},
			expectError: true,
			errorType:   db.ErrCreatingTenant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			if tt.forceError != nil {
				tt.forceError()
			}

			err := db.CreateSchema(ctx, database, tt.tenant)

			if tt.cleanup != nil {
				tt.cleanup()
			}

			if tt.expectError {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.errorType, "Expected %v, got %v", tt.errorType, err)

				return
			}

			require.NoError(t, err)
			integrationutils.TenantExists(t, database, tt.tenant.SchemaName, ExampleModel{}.TableName())
		})
	}
}

func TestBuildGroupName(t *testing.T) {
	tests := []struct {
		name         string
		tenantID     string
		groupType    string
		expectedName string
		expectedErr  error
	}{
		{
			name:         "Success_Admin",
			tenantID:     "tenant123",
			groupType:    constants.TenantAdminGroup,
			expectedName: "KMS_" + constants.TenantAdminGroup + "_tenant123",
			expectedErr:  nil,
		},
		{
			name:         "Success_Auditor",
			tenantID:     "tenant456",
			groupType:    constants.TenantAuditorGroup,
			expectedName: "KMS_" + constants.TenantAuditorGroup + "_tenant456",
			expectedErr:  nil,
		},
		{
			name:        "EmptyTenantID",
			tenantID:    "",
			groupType:   constants.TenantAdminGroup,
			expectedErr: db.ErrEmptyTenantID,
		},
		{
			name:        "InvalidGroupType",
			tenantID:    "tenant123",
			groupType:   "INVALID",
			expectedErr: db.ErrInvalidGroupType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, err := db.BuildIAMIdentifier(tt.groupType, tt.tenantID)
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedName, name)
			}
		})
	}
}
