package sqlinjection_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func startDB(t *testing.T) (*multitenancy.DB, string) {
	t.Helper()

	dbConfig := testutils.TestDBConfig{}
	db, tenants, _ := testutils.NewTestDB(t, dbConfig)

	return db, tenants[0]
}

func TestRepo_List_ForInjection(t *testing.T) {
	db, tenant := startDB(t)

	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	names := []string{"name1", "name2"}
	for _, name := range names {
		err := r.Create(ctx, &testutils.TestModel{ID: uuid.New(), Name: name})
		assert.NoError(t, err)
	}

	// Following result in SQL like:
	// SELECT count(*) FROM "test_models" WHERE name = XXX;
	// The XXXs are shown for the test strings in the accompanying comments below.
	// Tests show that ' appear to be sufficiently escaped
	attackStrings := []string{
		"');drop table test_models;",     // ('name1'');drop table test_models;')
		"');drop table \"test_models\";", // ('name1'');drop table "test_models";')
		"');drop table 'test_models';",   // ('name1'');drop table ''test_models'';')

		"');drop table \"test_models\";",   // ('name1'');drop table "test_models";')
		"'');drop table \"test_models\";",  // ('name1'''');drop table "test_models";')
		"\\');drop table \"test_models\";", // ('name1\'');drop table "test_models";')

		" OR 1=1",     // ('name1 OR 1=1')
		" OR '1'='1'", // ('name1 OR 1=1')
	}

	for _, attackString := range attackStrings {
		res := []*testutils.TestModel{}

		ck := repo.NewCompositeKey().Where("name", names[0]+attackString)
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
		count, err := r.List(ctx, testutils.TestModel{}, &res, query)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
		assert.Empty(t, res)
	}

	// Check table still exists
	res := []*testutils.TestModel{}
	ck := repo.NewCompositeKey().Where("name", names[0])
	query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
	count, err := r.List(ctx, testutils.TestModel{}, &res, query)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Len(t, res, 1)
}

func TestRepo_Create_ForInjection(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	// Following result in SQL like:
	// INSERT INTO "test_models" ("id","name","description","created_at","updated_at")
	// VALUES ('00000000-0000-0000-0000-000000000000','XXX','','2025-11-14 15:34:00.635','2025-11-14 15:34:00.635')
	// The XXXs are shown for the test strings in the accompanying comments below.
	// Tests show that ' appear to be sufficiently escaped
	attackStrings := []string{
		"');drop table test_models;",     // 8bbf0190-c076-406d-bf75-210fbc42b818'');drop table test_models;
		"');drop table \"test_models\";", // 592790e8-0089-4e23-9e63-d784f99b0f45'');drop table "test_models";
		"');drop table 'test_models';",   // 72e0645a-4264-4b05-9152-221b38c0becd'');drop table ''test_models'';

		"');drop table \"test_models\";",   // 82ba6f2d-b994-4f52-be82-bfa693e774b6'');drop table "test_models";
		"'');drop table \"test_models\";",  // 6844bf89-d384-40fb-aad6-221b9c86ee2b'''');drop table "test_models";
		"\\');drop table \"test_models\";", // 63414115-6de7-4635-a8d0-28cf63efc169\'');drop table "test_models";
	}

	for _, attackString := range attackStrings {
		item := testutils.TestModel{
			ID:   uuid.New(),
			Name: uuid.New().String() + attackString,
		}
		err := r.Create(ctx, &item)
		assert.NoError(t, err)

		// Check table still exists
		res := &testutils.TestModel{ID: item.ID}

		_, err = r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, item.ID, res.ID)
	}
}

func TestRepo_First_ForInjection(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	testModel := &testutils.TestModel{
		ID:        uuid.New(),
		Name:      uuid.New().String(),
		CreatedAt: time.Now(),
	}

	err := r.Create(ctx, testModel)
	assert.NoError(t, err)

	// Since we use literal string in the tests below we must ensure that the
	// tests are using the correct tenant table name. If this assertion starts
	// to fail we will also need to update the table names in the tests below.
	assert.Equal(t, "test_models", testutils.TestModelName)

	// Following result in SQL like:
	// SELECT * FROM "test_models" WHERE name = XXX ORDER BY "test_models"."id" LIMIT 1;
	// The XXXs are shown for the test strings in the accompanying comments below.
	// Tests show that ' appear to be sufficiently escaped

	attackStrings := []string{
		"');drop table test_models;",     // ('47683b68-43ab-41e5-82ab-4eb50113565f'');drop table test_models;')
		"');drop table \"test_models\";", // ('47683b68-43ab-41e5-82ab-4eb50113565f'');drop table "test_models";')
		"');drop table 'test_models';",   // ('47683b68-43ab-41e5-82ab-4eb50113565f'');drop table ''test_models'';')

		"');drop table \"test_models\";",   // ('47683b68-43ab-41e5-82ab-4eb50113565f'');drop table "test_models";')
		"'');drop table \"test_models\";",  // ('47683b68-43ab-41e5-82ab-4eb50113565f'''');drop table "test_models";')
		"\\');drop table \"test_models\";", // ('47683b68-43ab-41e5-82ab-4eb50113565f\'');drop table "test_models";')

		" OR 1=1",
		" OR '1'='1'",
	}

	for _, attackString := range attackStrings {
		res := &testutils.TestModel{Name: attackString}

		ck := repo.NewCompositeKey().Where("name", testModel.Name+attackString)
		query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
		ok, err := r.First(ctx, res, query)

		assert.Error(t, err)
		assert.False(t, ok)
		assert.ErrorIs(t, err, repo.ErrNotFound)
	}

	// Check table still exists
	res := &testutils.TestModel{Name: testModel.Name}
	ck := repo.NewCompositeKey().Where("name", testModel.Name)
	query := *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))
	_, err = r.First(ctx, res, query)

	assert.NoError(t, err)
}

func TestRepo_Delete_ForInjection(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	// Since we use literal string in the tests below we must ensure that the
	// tests are using the correct tenant table name. If this assertion starts
	// to fail we will also need to update the table names in the tests below.
	assert.Equal(t, "test_models", testutils.TestModelName)

	attackStrings := []string{
		"');drop table test_models;",
		"');drop table \"test_models\";",
		"');drop table 'test_models';",

		"');drop table \"test_models\";",
		"'');drop table \"test_models\";",
		"\\');drop table \"test_models\";",
	}

	for _, attackString := range attackStrings {
		item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String() + attackString}
		ok, err := r.Delete(ctx, &item, *repo.NewQuery())
		assert.NoError(t, err)
		assert.False(t, ok)
	}

	// Check the table still exists
	item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
	err := r.Create(ctx, &item)
	assert.NoError(t, err)
}

func TestRepo_Patch_ForInjection(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	// Since we use literal string in the tests below we must ensure that the
	// tests are using the correct tenant table name. If this assertion starts
	// to fail we will also need to update the table names in the tests below.
	assert.Equal(t, "test_models", testutils.TestModelName)

	attackStrings := []string{
		"');drop table test_models;",
		"');drop table \"test_models\";",
		"');drop table 'test_models';",

		"');drop table \"test_models\";",
		"'');drop table \"test_models\";",
		"\\');drop table \"test_models\";",
	}

	for _, attackString := range attackStrings {
		item := testutils.TestModel{
			ID:   uuid.New(),
			Name: uuid.New().String() + attackString,
		}
		ok, err := r.Patch(ctx, &item, *repo.NewQuery())
		assert.NoError(t, err)
		assert.False(t, ok)
	}

	// Check table still exists
	item := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
	err := r.Create(ctx, &item)
	assert.NoError(t, err)
}

func TestRepo_Set_ForInjection(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	// Since we use literal string in the tests below we must ensure that the
	// tests are using the correct tenant table name. If this assertion starts
	// to fail we will also need to update the table names in the tests below.
	assert.Equal(t, "test_models", testutils.TestModelName)

	attackStrings := []string{
		"');drop table test_models;",
		"');drop table \"test_models\";",
		"');drop table 'test_models';",

		"');drop table \"test_models\";",
		"'');drop table \"test_models\";",
		"\\');drop table \"test_models\";",
	}

	for _, attackString := range attackStrings {
		m := testutils.TestModel{ID: uuid.New(), Name: uuid.New().String()}
		err := r.Create(ctx, &m)
		assert.NoError(t, err)

		m.Name += attackString
		err = r.Set(ctx, &m)
		assert.NoError(t, err)

		// Check table still exists
		res := &testutils.TestModel{Name: "test" + attackString}
		ok, err := r.First(ctx, res, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
	}
}
