package postgres

import (
	"errors"
	"fmt"

	"github.com/avast/retry-go/v5"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy/driver"
	"github.com/openkcm/cmk/internal/multitenancy/gmterrors"
	"github.com/openkcm/cmk/internal/multitenancy/migrator"
)

// retryOptions builds the retry-go options from the dialector's configured retry settings.
func (m Migrator) retryOptions() []retry.Option {
	return []retry.Option{
		retry.Attempts(m.options.MaxRetries),
		retry.Delay(m.options.RetryDelay),
		retry.MaxDelay(m.options.MaxInterval),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	}
}

func (m Migrator) retry(fn func() error) error {
	if m.options.DisableRetry {
		return fn()
	}
	return retry.New(m.retryOptions()...).Do(fn)
}

// Migration errors.
var (
	ErrNoTenantTables = errors.New("no tenant tables to migrate")
	ErrNoPublicTables = errors.New("no public tables to migrate")
)

func (m Migrator) acquireXact(tx *gorm.DB, lockKey string) error {
	if m.options.DisableRetry {
		return acquireXact(tx, lockKey)
	}
	return acquireXact(tx, lockKey, m.retryOptions()...)
}

func (m Migrator) AutoMigrate(values ...any) error {
	_, err := migrator.OptionFromDB(m.DB)
	if err != nil {
		return gmterrors.NewWithScheme(DriverName, err)
	}
	return m.retry(func() error {
		return m.Migrator.AutoMigrate(values...)
	})
}

// MigrateTenantModels creates a schema for a specific tenant and migrates the private tables.
func (m Migrator) MigrateTenantModels(tenantID string) error {
	m.logger.Printf("migrating tables for tenant %s", tenantID)

	tenantModels := m.registry.TenantModels
	if len(tenantModels) == 0 {
		return gmterrors.NewWithScheme(DriverName, ErrNoTenantTables)
	}

	sqlstr := quoteRawSQLForTenant(m.DB, "CREATE SCHEMA IF NOT EXISTS ", tenantID)
	if err := m.DB.Exec(sqlstr).Error; err != nil {
		return gmterrors.NewWithScheme(DriverName,
			fmt.Errorf("failed to create schema for tenant %s: %w", tenantID, err))
	}

	err := m.DB.Transaction(func(tx *gorm.DB) error {
		if err := m.acquireXact(tx, tenantID); err != nil {
			return gmterrors.NewWithScheme(DriverName,
				fmt.Errorf("failed to acquire advisory lock for tenant %s: %w", tenantID, err))
		}
		reset, searchPathErr := SetSearchPath(tx, tenantID)
		if searchPathErr != nil {
			return gmterrors.NewWithScheme(DriverName,
				fmt.Errorf("failed to set search path to tenant %s: %w", tenantID, searchPathErr))
		}
		defer func() { _ = reset() }()

		if err := tx.
			Scopes(migrator.WithOption(migrator.MigratorOption)).
			AutoMigrate(driver.ModelsToInterfaces(tenantModels)...); err != nil {
			return gmterrors.NewWithScheme(DriverName,
				fmt.Errorf("failed to migrate private tables for tenant %s: %w", tenantID, err))
		}
		m.logger.Printf("private tables migrated for tenant %s", tenantID)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// MigrateSharedModels migrates the public tables in the database.
func (m Migrator) MigrateSharedModels() error {
	m.logger.Println("migrating public tables")

	publicModels := m.registry.SharedModels
	if len(publicModels) == 0 {
		return gmterrors.NewWithScheme(DriverName, ErrNoPublicTables)
	}

	tx := m.DB.Begin()
	defer func() {
		if tx.Error == nil {
			tx.Commit()
			m.logger.Printf("public tables migrated for all tenants")
		} else {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return gmterrors.NewWithScheme(DriverName, fmt.Errorf("failed to begin transaction: %w", err))
	}

	if err := m.acquireXact(tx, driver.PublicSchemaName()); err != nil {
		tx.Rollback()
		return gmterrors.NewWithScheme(DriverName, fmt.Errorf("failed to acquire advisory lock: %w", err))
	}

	if err := tx.
		Scopes(migrator.WithOption(migrator.MigratorOption)).
		AutoMigrate(driver.ModelsToInterfaces(publicModels)...); err != nil {
		tx.Rollback()
		return gmterrors.NewWithScheme(DriverName, fmt.Errorf("failed to migrate public tables: %w", err))
	}

	return tx.Commit().Error
}

// DropSchemaForTenant drops the schema for a specific tenant.
func (m Migrator) DropSchemaForTenant(tenant string) error {
	m.logger.Printf("dropping schema for tenant %s", tenant)

	sqlstr := quoteRawSQLForTenant(m.DB, "DROP SCHEMA IF EXISTS ", tenant) + " CASCADE"
	err := m.retry(func() error {
		if err := m.DB.Exec(sqlstr).Error; err != nil {
			return gmterrors.NewWithScheme(DriverName, fmt.Errorf("failed to drop schema for tenant %s: %w", tenant, err))
		}
		m.logger.Printf("schema dropped for tenant %s", tenant)
		return nil
	})

	return err
}
