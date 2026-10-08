// Package multitenancy provides PostgreSQL schema-per-tenant support on top of GORM.
//
// It wraps a [gorm.DB] connection with tenant-aware operations: registering shared and
// tenant-specific models, migrating them, switching the active tenant schema, and cleaning
// up a tenant's schema on offboarding. The concrete database behavior is provided by a
// registered [Adapter] (see the postgres subpackage).
package multitenancy

import (
	"context"
	"database/sql"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy/driver"
)

// DB wraps a GORM DB connection, integrating support for multitenancy operations.
// It provides a unified interface for managing tenant-specific and shared data within
// a multi-tenant application, leveraging GORM's ORM capabilities for database operations.
type DB struct {
	*gorm.DB

	driver driver.DBFactory
}

// NewDB creates a new [DB] instance using the provided [driver.DBFactory] and [gorm.DB]
// instance. This function is intended for use by [Adapter] implementations to create new
// instances of DB with multitenancy support. Not intended for direct use in application code.
func NewDB(d driver.DBFactory, tx *gorm.DB) *DB {
	return &DB{
		DB:     tx,
		driver: d,
	}
}

// CurrentTenant returns the identifier for the current tenant context or an empty string
// if no context is set.
func (db *DB) CurrentTenant(ctx context.Context) string {
	return db.driver.CurrentTenant(ctx, db.DB)
}

// RegisterModels registers GORM model structs for multitenancy support, preparing models for
// tenant-specific operations.
//
// Not safe for concurrent use by multiple goroutines. Call this method from your main function
// or during application initialization.
func (db *DB) RegisterModels(ctx context.Context, models ...driver.TenantTabler) error {
	return db.driver.RegisterModels(ctx, db.DB, models...)
}

// MigrateSharedModels migrates all registered shared/public models.
//
// Safe for concurrent use by multiple goroutines w.r.t. ensuring data integrity and schema isolation.
func (db *DB) MigrateSharedModels(ctx context.Context) error {
	return db.driver.MigrateSharedModels(ctx, db.DB)
}

// MigrateTenantModels migrates all registered tenant-specific models for the specified tenant.
// This method is intended to be used when onboarding a new tenant or updating an existing tenant's
// schema to match the latest model definitions.
//
// Safe for concurrent use by multiple goroutines w.r.t. ensuring data integrity and schema isolation.
func (db *DB) MigrateTenantModels(ctx context.Context, tenantID string) error {
	return db.driver.MigrateTenantModels(ctx, db.DB, tenantID)
}

// OffboardTenant cleans up the database by dropping the tenant-specific schema and associated tables.
// This method is intended to be used after a tenant has been removed.
//
// Safe for concurrent use by multiple goroutines w.r.t. ensuring data integrity and schema isolation.
func (db *DB) OffboardTenant(ctx context.Context, tenantID string) error {
	return db.driver.OffboardTenant(ctx, db.DB, tenantID)
}

// UseTenant configures the database for operations specific to a tenant. A reset function is returned
// to revert the database context to its original state.
//
// Technically safe for concurrent use by multiple goroutines, but should not be used concurrently
// w.r.t. ensuring data integrity and schema isolation. Either use [DB.WithTenant], or ensure that this
// method is called within a transaction or from its own database connection.
//
//nolint:nonamedreturns // named returns document the (reset, err) contract callers defer on
func (db *DB) UseTenant(ctx context.Context, tenantID string) (reset func() error, err error) {
	return db.driver.UseTenant(ctx, db.DB, tenantID)
}

// WithTenant executes the provided function within the context of a specific tenant, ensuring that
// the database operations are scoped to the tenant's schema. It runs in a transaction that is
// committed on success and rolled back on error.
//
// Safe for concurrent use by multiple goroutines w.r.t. ensuring data integrity and schema isolation.
func (db *DB) WithTenant(
	ctx context.Context,
	tenantID string,
	fc func(tx *DB) error,
	opts ...*sql.TxOptions,
) (err error) {
	tx := db.WithContext(ctx).Begin(opts...)
	defer func() {
		if tx.Error == nil {
			err = tx.Commit().Error
		} else {
			err = tx.Rollback().Error
		}
	}()

	reset, err := tx.UseTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() { _ = reset() }()
	return fc(NewDB(db.driver, tx.DB))
}

// ======================================================================================
// The below methods have been overridden to return a new DB instance with the updated
// configuration, allowing for method chaining and preserving the multitenancy context.
// ======================================================================================

// Session returns a new copy of the DB, which has a new session with the configuration.
func (db *DB) Session(config *gorm.Session) *DB {
	return NewDB(db.driver, db.DB.Session(config))
}

// WithContext sets the context for the DB.
func (db *DB) WithContext(ctx context.Context) *DB {
	return NewDB(db.driver, db.DB.WithContext(ctx))
}

// Transaction starts a transaction as a block, returns an error if there's any error
// within the block. If the function passed to tx returns an error, the transaction will
// be rolled back automatically, otherwise, the transaction will be committed.
//
//nolint:nonamedreturns // mirrors gorm.DB.Transaction's named-return signature
func (db *DB) Transaction(fc func(tx *DB) error, opts ...*sql.TxOptions) (err error) {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return fc(NewDB(db.driver, tx))
	}, opts...)
}

// Begin begins a transaction.
func (db *DB) Begin(opts ...*sql.TxOptions) *DB {
	return NewDB(db.driver, db.DB.Begin(opts...))
}
