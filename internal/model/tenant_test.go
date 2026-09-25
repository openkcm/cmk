package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestTenantsTable(t *testing.T) {
	t.Run("Should have table name tenants", func(t *testing.T) {
		expectedTableName := "public.tenants"

		tableName := model.Tenant{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a public table", func(t *testing.T) {
		assert.True(t, model.Tenant{}.IsSharedModel())
	})

	t.Run("Should have unique combination id and region", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})

		r := sql.NewRepository(db)
		err := r.Create(t.Context(), &model.Tenant{
			ID:         "test-id",
			DomainURL:  "test-domain.example.com",
			SchemaName: "test-schema",
		})
		assert.NoError(t, err)

		err = r.Create(t.Context(), &model.Tenant{
			ID:         "test-id1",
			DomainURL:  "test-domain1.example.com",
			SchemaName: "test-schema1",
		})
		assert.NoError(t, err)
	})
}

func TestTenantValidate(t *testing.T) {
	validStatus := model.TenantStatus(pb.Status_STATUS_ACTIVE.String())
	validRole := model.TenantRole(pb.Role_ROLE_LIVE.String())
	limit := func(v int) *int { return &v }

	tests := map[string]struct {
		override  *int
		expectErr error
	}{
		"no override":        {override: nil},
		"override at min":    {override: limit(1)},
		"override above min": {override: limit(50)},
		"override zero":      {override: limit(0), expectErr: model.ErrInvalidSystemLimitOverride},
		"override negative":  {override: limit(-1), expectErr: model.ErrInvalidSystemLimitOverride},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tenant := model.Tenant{
				Status:              validStatus,
				Role:                validRole,
				SystemLimitOverride: test.override,
			}

			err := tenant.Validate()
			if test.expectErr != nil {
				assert.ErrorIs(t, err, test.expectErr)
				assert.ErrorIs(t, err, model.ErrValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
