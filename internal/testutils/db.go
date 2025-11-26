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

var TestModelName = "test_models"

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
	return TestModelName
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
// It returns a pointer to the multitenancy.DB instance, a slice of tenant IDs and it's config.
// By default, it uses TestDB configuration. Use opts to customize the setup.
// This function is intended for use in unit tests.
//
//nolint:funlen
func NewTestDB(tb testing.TB, cfg TestDBConfig, opts ...TestDBConfigOpt) (*multitenancy.DB, []string, config.Database) {
	tb.Helper()

	cfg.dbCon = TestDB

	cfg.generateTenants = 1
	for _, o := range opts {
		o(&cfg)
	}

	db := newTestDBCon(tb, &cfg)

	tb.Cleanup(func() {
		sqlDB, _ := db.DB.DB()
		sqlDB.Close()
	})

	tenantIDs := make([]string, 0, max(cfg.generateTenants, len(cfg.initTenants)))

	// Return instance with only init tenants
	if len(cfg.initTenants) > 0 {
		for _, tenant := range cfg.initTenants {
			createTenant(tb, db, &tenant, cfg.Models)
			tenantIDs = append(tenantIDs, tenant.ID)
		}

		return db, tenantIDs, cfg.dbCon
	}

	if cfg.CreateDatabase {
		for i := range cfg.generateTenants {
			schema := processNameForDB(fmt.Sprintf("tenant%d", i))
			tenant := NewTenant(func(t *model.Tenant) {
				t.SchemaName = schema
				t.DomainURL = schema + ".example.com"
				t.ID = schema
				t.OwnerID = schema + "-owner-id"
			})
			createTenant(tb, db, tenant, cfg.Models)
			tenantIDs = append(tenantIDs, tenant.ID)
		}
	} else {
		schema := processNameForDB(tb.Name())
		tenant := NewTenant(func(t *model.Tenant) {
			t.SchemaName = schema
			t.DomainURL = schema + ".example.com"
			t.ID = schema
			t.OwnerID = schema + "-owner-id"
		})
		createTenant(tb, db, tenant, cfg.Models)
		tenantIDs = append(tenantIDs, tenant.ID)
	}

	if cfg.WithOrbital {
		schema := "orbital"
		tenant := NewTenant(func(t *model.Tenant) {
			t.SchemaName = schema
			t.DomainURL = schema + ".example.com"
			t.ID = schema
			t.OwnerID = schema + "-owner-id"
		})
		createTenant(tb, db, tenant, cfg.Models)
	}

	return db, tenantIDs, cfg.dbCon
}

func createTenant(tb testing.TB, db *multitenancy.DB, tenant *model.Tenant, m []driver.TenantTabler) {
	tb.Helper()

	tb.Cleanup(func() {
		_ = db.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE schema_name = '%s';", model.Tenant{}.TableName(), tenant.SchemaName),
		)
		err := db.OffboardTenant(tb.Context(), tenant.SchemaName)
		assert.NoError(tb, err)
	})

	m = append(m, tenant)

	if len(m) > 1 {
		require.NoError(tb, db.RegisterModels(tb.Context(), m...))
		require.NoError(tb, db.MigrateSharedModels(tb.Context()))
		require.NoError(tb, db.Create(tenant).Error)
		require.NoError(tb, db.MigrateTenantModels(tb.Context(), tenant.SchemaName))

		// Clean keystore_configurations table if it's included in models
		for _, table := range m {
			if _, ok := table.(*model.KeystoreConfiguration); ok {
				err := db.Exec("DELETE FROM keystore_configurations").Error
				assert.NoError(tb, err)

				break
			}
		}
	}
}

func WithDatabase(db config.Database) TestDBConfigOpt {
	return func(c *TestDBConfig) {
		c.dbCon = db
	}
}

// WithInitTenants creates the provided tenants on the DB
// No default tenants are generated on provided tenants
func WithInitTenants(tenants ...model.Tenant) TestDBConfigOpt {
	return func(c *TestDBConfig) {
		c.initTenants = tenants
		c.CreateDatabase = true
	}
}

// WithGenerateTenants creates count tenants on a separate database
func WithGenerateTenants(count int) TestDBConfigOpt {
	return func(c *TestDBConfig) {
		c.generateTenants = count
		c.CreateDatabase = true
	}
}

type TestDBConfig struct {
	dbCon config.Database

	// Generate N tenants
	generateTenants int

	// This option should be used to create determinated tenants
	// If Generate Tenants is set to 0 and no InitTenants are provided, one is created
	initTenants []model.Tenant

	// WithOrbital creates an entry for an orbital tenant
	// This should only be used in tests where we want to access orbital table entries with the repo interface
	WithOrbital bool

	// If true create DB instance for test instead of tenant
	// This should be used whenever each test is testing either:
	// - Shared Tables
	// - Multiple Tenants
	CreateDatabase bool

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
// to setup a DB for unit tests
func newTestDBCon(tb testing.TB, cfg *TestDBConfig) *multitenancy.DB {
	tb.Helper()

	if !cfg.CreateDatabase {
		con, err := db.StartDBConnection(
			cfg.dbCon,
			[]config.Database{},
		)
		assert.NoError(tb, err)

		return con
	}

	cfg.dbCon = NewIsolatedDB(tb, cfg.dbCon)

	con, err := db.StartDBConnection(
		cfg.dbCon,
		[]config.Database{},
	)
	assert.NoError(tb, err)

	return con
}

// NewIsolatedDB creates a new database on a postgres instance and returns it
//
// This is intended only for tests that call functions establishing DB connection
func NewIsolatedDB(tb testing.TB, cfg config.Database) config.Database {
	tb.Helper()

	con, err := db.StartDBConnection(
		cfg,
		[]config.Database{},
	)
	assert.NoError(tb, err)

	name := processNameForDB(tb.Name())
	assert.NoError(tb, err)

	// No need to t.CleanUp as it only throws error on db error
	err = con.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", name)).Error
	assert.NoError(tb, err)
	err = con.Exec(fmt.Sprintf("CREATE DATABASE %s;", name)).Error
	assert.NoError(tb, err)

	cfg.Name = name

	return cfg
}
