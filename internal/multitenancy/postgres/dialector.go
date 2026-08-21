package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	gormmigrator "gorm.io/gorm/migrator"

	"github.com/openkcm/cmk/internal/multitenancy/driver"
	"github.com/openkcm/cmk/internal/multitenancy/gmterrors"
	"github.com/openkcm/cmk/internal/multitenancy/logext"
)

// Default retry configuration for migrations.
const (
	defaultMaxRetries  = 6
	defaultRetryDelay  = 2 * time.Second
	defaultMaxInterval = 30 * time.Second
)

type (
	// Options provides configuration options with multitenancy support.
	// By default, retry is enabled. To disable retry, set DisableRetry to true.
	// Note that the retry logic is only applied to migrations.
	Options struct {
		DisableRetry bool          // Whether to disable retry.
		MaxRetries   uint          // Maximum retry attempts.
		RetryDelay   time.Duration // Initial delay between retries.
		MaxInterval  time.Duration // Maximum delay between retries.
	}

	// Option is a function that modifies an [Options] instance.
	Option func(*Options)

	// Config provides configuration with multitenancy support.
	Config struct {
		postgres.Config
	}

	// Dialector provides a dialector with multitenancy support.
	//
	//nolint:recvcheck // Migrator() uses a value receiver (gorm calls it on a value); other methods need a pointer
	Dialector struct {
		*postgres.Dialector

		registry *driver.ModelRegistry
		logger   *logext.Logger
		options  *Options
	}

	// Migrator provides a migrator with multitenancy support.
	Migrator struct {
		postgres.Migrator
		Dialector
	}
)

func (o *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}

	if !o.DisableRetry {
		o.MaxRetries = max(o.MaxRetries, defaultMaxRetries)
		o.RetryDelay = max(o.RetryDelay, defaultRetryDelay)
		o.MaxInterval = max(o.MaxInterval, defaultMaxInterval)
	}
}

var _ gorm.Dialector = new(Dialector)

// New creates a new PostgreSQL dialector with multitenancy support.
func New(config Config, opts ...Option) gorm.Dialector {
	options := &Options{}
	options.apply(opts...)
	return &Dialector{
		//nolint:forcetypeassert // gorm's postgres.New always returns *postgres.Dialector
		Dialector: postgres.New(config.Config).(*postgres.Dialector),
		registry:  &driver.ModelRegistry{},
		logger:    logext.Default(),
		options:   options,
	}
}

// Migrator returns a [gorm.Migrator] implementation for the Dialector.
func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return &Migrator{
		postgres.Migrator{
			Migrator: gormmigrator.Migrator{
				Config: gormmigrator.Config{
					DB:                          db,
					Dialector:                   dialector,
					CreateIndexAfterCreateTable: true,
				},
			},
		},
		dialector,
	}
}

// RegisterModels registers the given models with the dialector for multitenancy support.
func (dialector *Dialector) RegisterModels(models ...driver.TenantTabler) error {
	registry, err := driver.NewModelRegistry(models...)
	if err != nil {
		return gmterrors.NewWithScheme(DriverName, fmt.Errorf("failed to register models: %w", err))
	}
	dialector.registry = registry
	return nil
}

// RegisterModels registers the given models with the provided [gorm.DB] instance for multitenancy support.
// Not safe for concurrent use by multiple goroutines.
func RegisterModels(db *gorm.DB, models ...driver.TenantTabler) error {
	//nolint:forcetypeassert // db is always opened with this package's Dialector
	return db.Dialector.(*Dialector).RegisterModels(models...)
}

// MigratePublicSchema migrates the public schema in the database.
func MigratePublicSchema(db *gorm.DB) error {
	return db.Connection(func(tx *gorm.DB) error {
		//nolint:forcetypeassert // this dialector always builds a *Migrator
		return tx.Migrator().(*Migrator).MigrateSharedModels()
	})
}

// MigrateTenantModels creates a new schema for a specific tenant in the PostgreSQL database.
func MigrateTenantModels(db *gorm.DB, schemaName string) error {
	return db.Connection(func(tx *gorm.DB) error {
		//nolint:forcetypeassert // this dialector always builds a *Migrator
		return tx.Migrator().(*Migrator).MigrateTenantModels(schemaName)
	})
}

// DropSchemaForTenant drops the schema for a specific tenant in the PostgreSQL database (CASCADE).
func DropSchemaForTenant(db *gorm.DB, schemaName string) error {
	//nolint:forcetypeassert // this dialector always builds a *Migrator
	return db.Migrator().(*Migrator).DropSchemaForTenant(schemaName)
}
