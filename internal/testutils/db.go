package testutils

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	TestTenant = "test"
)

type PublicTestModel struct{}

func (PublicTestModel) IsSharedModel() bool {
	return true
}

func (PublicTestModel) TableName() string {
	return "public.test_models"
}

// TestModel represents a model for testing Migration and CRUD operations
type TestModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"type:varchar(255);unique"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Related     []TestRelatedModel `gorm:"foreignKey:TestModelID"`
}

func (TestModel) TableName() string {
	return "test_models"
}

func (TestModel) IsSharedModel() bool {
	return false
}

// TestRelatedModel represents a model for testing preload functionality
type TestRelatedModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	TestModelID uuid.UUID `gorm:"type:uuid"`
	Name        string    `gorm:"type:varchar(255)"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TestRelatedModel) TableName() string {
	return "test_related_models"
}

func (TestRelatedModel) IsSharedModel() bool {
	return false
}

func CreateCtxWithTenant(tenant string) context.Context {
	return context.WithValue(context.Background(), nethttp.TenantKey, tenant)
}

func WithTenantID(ctx context.Context, db *multitenancy.DB, tenantID string, fn func(tx *multitenancy.DB) error) error {
	var existingTenant model.Tenant

	err := db.Where(repo.IDField+" = ?", tenantID).First(&existingTenant).Error
	if err != nil {
		return fmt.Errorf("tenant with ID %s not found: %w", tenantID, err)
	}

	return db.WithTenant(ctx, existingTenant.SchemaName, fn)
}

func CreateTestEntities(ctx context.Context, tb testing.TB, r repo.Repo, entities ...repo.Resource) {
	tb.Helper()

	for _, e := range entities {
		err := r.Create(ctx, e)
		assert.NoError(tb, err)
	}
}

func DeleteTestEntities(ctx context.Context, tb testing.TB, r repo.Repo, entities ...repo.Resource) {
	tb.Helper()

	for _, e := range entities {
		_, err := r.Delete(ctx, e, *repo.NewQuery())
		assert.NoError(tb, err)
	}
}

// RunTestQuery runs a query in the database with the specified tenant context
func RunTestQuery(db *multitenancy.DB, tenant string, queries ...string) {
	for _, query := range queries {
		_ = WithTenantID(CreateCtxWithTenant(tenant), db, tenant, func(tx *multitenancy.DB) error {
			return tx.Exec(query).Error
		})
	}
}

var TestDB = config.Database{
	Host: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "localhost",
	},
	User: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "postgres",
	},
	Secret: commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  "secret",
	},
	Name: "cmk",
	Port: "5433",
}

type TestDBConfigOpt func(*TestDBConfig)

// NewTestDB sets up a test database connection and creates tenants as needed.
// It returns a pointer to the multitenancy.DB instance and a slice of tenant IDs.
// By default, it uses TestDB configuration. Use opts to customize the setup.
// This function is intended for use in unit tests.
func NewTestDB(tb testing.TB, cfg TestDBConfig, opts ...TestDBConfigOpt) (*multitenancy.DB, []string) {
	tb.Helper()

	cfg.dbCon = TestDB
	for _, o := range opts {
		o(&cfg)
	}

	db := newTestDBCon(tb, cfg)

	tb.Cleanup(func() {
		sqlDB, _ := db.DB.DB()
		sqlDB.Close()
	})

	tenantIDs := make([]string, 0, cfg.TenantCount)
	if cfg.RequiresMultitenancyOrShared {
		if cfg.TenantCount <= 1 {
			cfg.TenantCount = 1
		}

		for i := range cfg.TenantCount {
			schema := processNameForDB(fmt.Sprintf("tenant%d", i))
			tenantID := newSchema(tb, db, schema, cfg.Models)
			tenantIDs = append(tenantIDs, tenantID)
		}
	} else {
		schema := processNameForDB(tb.Name())
		tenantID := newSchema(tb, db, schema, cfg.Models)
		tenantIDs = append(tenantIDs, tenantID)
	}

	return db, tenantIDs
}

func newSchema(tb testing.TB, db *multitenancy.DB, schema string, m []driver.TenantTabler) string {
	tb.Helper()

	_ = db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE schema_name = '%s';", model.Tenant{}.TableName(), schema),
	)
	_ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema))

	tenant := model.Tenant{
		TenantModel: multitenancy.TenantModel{
			DomainURL:  schema + ".example.com",
			SchemaName: schema,
		},
		ID:        uuid.NewString(),
		Region:    uuid.NewString(),
		Status:    "STATUS_ACTIVE",
		OwnerType: "test",
		OwnerID:   "test",
	}

	m = append(m, tenant)

	if len(m) > 1 {
		require.NoError(tb, db.RegisterModels(tb.Context(), m...))
		require.NoError(tb, db.MigrateSharedModels(tb.Context()))
		require.NoError(tb, db.Create(tenant).Error)
		require.NoError(tb, db.MigrateTenantModels(tb.Context(), schema))

		// Clean keystore_configurations table if it's included in models
		for _, table := range m {
			if _, ok := table.(*model.KeystoreConfiguration); ok {
				err := db.Exec("DELETE FROM keystore_configurations").Error
				assert.NoError(tb, err)

				break
			}
		}
	}

	return tenant.ID
}

func WithDatabase(db config.Database) TestDBConfigOpt {
	return func(c *TestDBConfig) {
		c.dbCon = db
	}
}

type TestDBConfig struct {
	dbCon config.Database

	// Only needs to be set for RequiresMultitenancyOrShared with multiple tenants
	// By default it creates only one tenant
	TenantCount int

	// If true create db instance for test instead of tenant
	RequiresMultitenancyOrShared bool

	// Tables that the test should contain
	Models []driver.TenantTabler
}

const MaxPSQLSchemaName = 64

// tb.Name() returns following format TESTA/SUBTESTB
// Postgres does not support schemas with "/" character and has max len 63 char
func processNameForDB(n string) string {
	name := strings.ToLower(n)
	name = strings.ReplaceAll(name, "/", "_")

	name = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(name, "")
	if len(name) >= MaxPSQLSchemaName {
		name = name[:MaxPSQLSchemaName-1]
	}

	return name
}

// newTestDBCon gets a PostgreSQL instance for the tests
// If cfg.RequiresMultitenancy create a separate database to test multitenancy
//
// This is intended for internal use. In most cases please use NewTestDB
// to setup a db for unit tests
func newTestDBCon(tb testing.TB, cfg TestDBConfig) *multitenancy.DB {
	tb.Helper()

	con, err := db.StartDBConnection(
		cfg.dbCon,
		[]config.Database{},
	)
	assert.NoError(tb, err)

	if !cfg.RequiresMultitenancyOrShared {
		return con
	}

	name := processNameForDB(tb.Name())
	assert.NoError(tb, err)

	// No need to t.CleanUp as it only throws error on db error
	err = con.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", name)).Error
	assert.NoError(tb, err)
	err = con.Exec(fmt.Sprintf("CREATE DATABASE %s;", name)).Error
	assert.NoError(tb, err)

	cfg.dbCon.Name = name

	con, err = db.StartDBConnection(
		cfg.dbCon,
		[]config.Database{},
	)
	assert.NoError(tb, err)

	return con
}
