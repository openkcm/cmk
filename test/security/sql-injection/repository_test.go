package sqlinjection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestRepository_ForInjection(t *testing.T) {
	db, tenant := startDB(t)
	r := sql.NewRepository(db)

	// Since we use literal string in the tests below we must ensure that the
	// tests are using the correct tenant table name. If this assertion starts
	// to fail we will also need to update the table names in the tests below.
	assert.Equal(t, "public.tenants", model.TenantTableName)

	// Following result in SQL like:
	//  SELECT * FROM "public"."tenants" WHERE id = XXX ORDER BY "tenants"."id" LIMIT 1
	// The XXXs are shown for the test strings in the accompanying comments below.
	// Tests show that ' appear to be sufficiently escaped
	attackStrings := []string{
		"');drop table public.tenants;",
		// ('fe2b5d0d-3eaf-49bb-a74a-187ce9a49924'');drop table public.tenants;')

		"');drop table \"public\".\"tenants\";",
		// ('fe2b5d0d-3eaf-49bb-a74a-187ce9a49924'');drop table "public"."tenants";')

		"');drop table 'public'.'tenants';",
		// ('bf74285f-2b11-4711-a40e-1df4ef9517f8'');drop table ''public''.''tenants'';'

		"');drop table \"public\".\"tenants\";",
		// ('4ed88fb7-e307-4a15-9ad0-175c5096dcb6'');drop table "public"."tenants";')

		"'');drop table \"public\".\"tenants\";",
		// ('4ed88fb7-e307-4a15-9ad0-175c5096dcb6'''');drop table "public"."tenants";')

		"\\');drop table \"public\".\"tenants\";",
		// ('4ed88fb7-e307-4a15-9ad0-175c5096dcb6\'');drop table "public"."tenants";')
	}

	for _, attackString := range attackStrings {
		ctx := testutils.CreateCtxWithTenant(tenant + attackString)
		gotTenant, err := repo.GetTenant(ctx, r)

		assert.Error(t, err)
		assert.ErrorIs(t, err, repo.ErrTenantNotFound)
		assert.Nil(t, gotTenant)

		// Ensure we still have a tenant table
		ctx = testutils.CreateCtxWithTenant(tenant)
		gotTenant, err = repo.GetTenant(ctx, r)

		assert.NoError(t, err)
		assert.NotNil(t, gotTenant)
		assert.Equal(t, tenant, gotTenant.ID)
	}
}
