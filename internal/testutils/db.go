package testutils

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	TestTenant = "test"
)

var TestModelName = "test_models"

// TestModel represents a model for testing Migration and CRUD operations
type TestModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"type:varchar(255);unique"`
	Description string
	Data        json.RawMessage `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TestModel) TableName() string {
	return TestModelName
}

func (TestModel) IsSharedModel() bool {
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

var (
	oncePostgres sync.Once
	dbCfg        config.Database
)

// NewTestDB sets up a test database connection and creates tenants as needed.
// It returns a pointer to the multitenancy.DB instance, a slice of tenant IDs and it's config.
// By default, it uses TestDB configuration. Use opts to customize the setup.
// This function is intended for use in unit tests.
//
//nolint:funlen,cyclop
func NewTestDB(tb testing.TB, cfg TestDBConfig, opts ...TestDBConfigOpt) (*multitenancy.DB, []string, config.Database) {
	tb.Helper()

	cfg.dbCon = TestDB

	_, filename, _, _ := runtime.Caller(0) //nolint: dogsled
	migrationPath := filepath.Join(filepath.Dir(filename), "../../migrations")

	cfg.dbCon.Migrator = config.Migrator{
		Shared: config.MigrationPath{
			Schema: filepath.Join(migrationPath, "/shared/schema"),
		},
		Tenant: config.MigrationPath{
			Schema: filepath.Join(migrationPath, "/tenant/schema"),
		},
	}

	cfg.generateTenants = 1
	for _, o := range opts {
		o(&cfg)
	}

	if !cfg.WithIsolatedService {
		oncePostgres.Do(func() {
			StartPostgresSQL(tb, &cfg.dbCon, testcontainers.WithReuseByName(uuid.NewString()))
			dbCfg = cfg.dbCon
		})
		cfg.dbCon = dbCfg
	} else {
		StartPostgresSQL(tb, &cfg.dbCon)
	}

	dbCon := newTestDBCon(tb, &cfg)

	migrator, err := db.NewMigrator(sql.NewRepository(dbCon), &config.Config{Database: cfg.dbCon})
	assert.NoError(tb, err)

	runMigration(tb, cfg, migrator, db.SharedTarget)

	if cfg.Logger != nil {
		dbCon = dbCon.Session(&gorm.Session{
			Logger: cfg.Logger,
		})
	}

	tenantIDs := make([]string, 0, max(cfg.generateTenants, len(cfg.initTenants)))

	// Return instance with only init tenants
	if len(cfg.initTenants) > 0 {
		for _, tenant := range cfg.initTenants {
			CreateDBTenant(tb, dbCon, &tenant)
			tenantIDs = append(tenantIDs, tenant.ID)
		}

		return dbCon, tenantIDs, cfg.dbCon
	}

	if cfg.CreateDatabase {
		for i := range cfg.generateTenants {
			schema := processNameForDB(fmt.Sprintf("tenant%d", i))
			tenant := NewTenant(func(t *model.Tenant) {
				t.SchemaName = schema
				t.DomainURL = schema + ".example.com"
				t.ID = schema
				t.OwnerID = schema + "-owner-id"
				if cfg.TenantRole != "" {
					t.Role = cfg.TenantRole
				}
			})
			CreateDBTenant(tb, dbCon, tenant)
			tenantIDs = append(tenantIDs, tenant.ID)
		}
	} else {
		schema := processNameForDB(tb.Name())
		tenant := NewTenant(func(t *model.Tenant) {
			t.SchemaName = schema
			t.DomainURL = schema + ".example.com"
			t.ID = schema
			t.OwnerID = schema + "-owner-id"
			if cfg.TenantRole != "" {
				t.Role = cfg.TenantRole
			}
		})
		CreateDBTenant(tb, dbCon, tenant)
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
		CreateDBTenant(tb, dbCon, tenant)
	}

	runMigration(tb, cfg, migrator, db.TenantTarget)

	return dbCon, tenantIDs, cfg.dbCon
}

func runMigration(
	tb testing.TB,
	cfg TestDBConfig,
	migrator db.Migrator,
	target db.MigrationTarget,
) {
	tb.Helper()

	req := db.Migration{
		Type: db.SchemaMigration,
	}
	switch target {
	case db.SharedTarget:
		req.Target = db.SharedTarget
	case db.TenantTarget:
		req.Target = db.TenantTarget
	default:
	}

	var version *int64
	switch target {
	case db.SharedTarget:
		version = cfg.SharedVersion
	case db.TenantTarget:
		version = cfg.TenantVersion
	default:
	}

	// Not set, migrate to latest
	if version == nil {
		err := migrator.MigrateToLatest(tb.Context(), req)
		assert.NoError(tb, err)
		return
	}

	if *version != 0 {
		err := migrator.MigrateTo(tb.Context(), req, *version)
		assert.NoError(tb, err)
	} else {
		return
	}
}

func CreateDBTenant(
	tb testing.TB,
	dbCon *multitenancy.DB,
	tenant *model.Tenant,
) {
	tb.Helper()

	tb.Cleanup(func() {
		_ = dbCon.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE schema_name = '%s';", model.Tenant{}.TableName(), tenant.SchemaName),
		)
		err := dbCon.OffboardTenant(context.Background(), tenant.SchemaName)
		assert.NoError(tb, err)
	})

	assert.NoError(tb, dbCon.Create(&tenant).Error)

	assert.NoError(tb, dbCon.RegisterModels(tb.Context(), &TestModel{}))
	assert.NoError(tb, dbCon.MigrateTenantModels(tb.Context(), tenant.ID))
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
		if count == 0 {
			c.TenantVersion = ptr.PointTo(int64(0))
		}
	}
}

// WithTenantRole specifies the role for generated tenants
// If not set, defaults to ROLE_LIVE
func WithTenantRole(role model.TenantRole) TestDBConfigOpt {
	return func(c *TestDBConfig) {
		c.TenantRole = role
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

	// If true create an isolated PSQL instance
	// In most cases this should not be set as it will take a longer time
	// as the container needs to build and startup
	WithIsolatedService bool

	// Shared schema version to migrate up to
	// If it's nil migrate to latest version
	SharedVersion *int64

	// Tenant schema version to migrate up to
	// If it's nil migrate to latest version
	TenantVersion *int64

	// TenantRole specifies the role for generated tenants
	// If empty, defaults to ROLE_LIVE
	TenantRole model.TenantRole

	// GORM Logger
	Logger logger.Interface
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

	if cfg.CreateDatabase {
		cfg.dbCon = NewIsolatedDB(tb, cfg.dbCon)
	}

	// Create new context so cleanup functions execute
	ctx := context.Background()

	con, err := db.StartDBConnection(
		ctx,
		cfg.dbCon,
		[]config.Database{},
	)
	assert.NoError(tb, err)

	tb.Cleanup(func() {
		sqlDB, _ := con.DB.DB()
		sqlDB.Close()
	})

	return con
}

// NewIsolatedDB creates a new database on a postgres instance and returns it
//
// This is intended only for tests that call functions establishing DB connection
func NewIsolatedDB(tb testing.TB, cfg config.Database) config.Database {
	tb.Helper()

	con, err := db.StartDBConnection(
		tb.Context(),
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

type migrator struct{}

func NewMigrator() db.Migrator {
	return &migrator{}
}

func (m *migrator) MigrateTenantToLatest(ctx context.Context, tenant *model.Tenant) error {
	return nil
}

func (m *migrator) MigrateToLatest(ctx context.Context, migration db.Migration) error {
	return nil
}

func (m *migrator) MigrateTo(ctx context.Context, migration db.Migration, version int64) error {
	return nil
}
