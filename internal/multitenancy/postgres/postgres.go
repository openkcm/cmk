// Package postgres provides a [gorm.Dialector] implementation for PostgreSQL databases
// to support multitenancy in GORM applications, enabling tenant-specific operations and
// shared resources management using the "shared database, separate schemas" approach.
//
// To ensure data integrity and schema isolation across tenants, [gorm.DB.AutoMigrate] is
// disabled; use [MigratePublicSchema] and [MigrateTenantModels] instead. Tenant migrations
// are guarded by PostgreSQL transaction advisory locks so only one migration runs at a time,
// with exponential-backoff retry enabled by default.
package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy"
	"github.com/openkcm/cmk/internal/multitenancy/driver"
)

// DriverName is the name of the PostgreSQL driver.
const DriverName = "postgres"

var (
	_ multitenancy.Adapter = new(postgresAdapter)
	_ driver.DBFactory     = new(postgresAdapter)
)

// postgresAdapter is a PostgreSQL-specific implementation of the [driver.DBFactory] interface.
type postgresAdapter struct{}

func init() { //nolint:gochecknoinits // Required for driver registration.
	multitenancy.Register(DriverName, &postgresAdapter{})
	multitenancy.Register("postgresql", &postgresAdapter{}) // Alias for postgres driver.
}

// AdaptDB implements [multitenancy.Adapter].
func (p *postgresAdapter) AdaptDB(_ context.Context, db *gorm.DB) (*multitenancy.DB, error) {
	return multitenancy.NewDB(&postgresAdapter{}, db), nil
}

// MigrateSharedModels implements [driver.DBFactory].
func (p *postgresAdapter) MigrateSharedModels(_ context.Context, db *gorm.DB) error {
	return MigratePublicSchema(db)
}

// MigrateTenantModels implements [driver.DBFactory].
func (p *postgresAdapter) MigrateTenantModels(_ context.Context, db *gorm.DB, tenantID string) error {
	return MigrateTenantModels(db, tenantID)
}

// OffboardTenant implements [driver.DBFactory].
func (p *postgresAdapter) OffboardTenant(_ context.Context, db *gorm.DB, tenantID string) error {
	return DropSchemaForTenant(db, tenantID)
}

// RegisterModels implements [driver.DBFactory].
func (p *postgresAdapter) RegisterModels(_ context.Context, db *gorm.DB, models ...driver.TenantTabler) error {
	return RegisterModels(db, models...)
}

// UseTenant implements [driver.DBFactory].
func (p *postgresAdapter) UseTenant(_ context.Context, db *gorm.DB, tenantID string) (func() error, error) {
	return SetSearchPath(db, tenantID)
}

// CurrentTenant implements [driver.DBFactory].
func (p *postgresAdapter) CurrentTenant(_ context.Context, db *gorm.DB) string {
	return CurrentSearchPath(db)
}
