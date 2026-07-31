// Package driver provides the foundational interfaces for implementing multitenancy support
// within database systems. It outlines the necessary components for managing tenant lifecycles,
// including onboarding, offboarding, and handling shared resources. These interfaces serve as
// a contract for database management systems (DBMS) to ensure consistent multitenant operations,
// abstracting the complexities of tenant-specific data handling.
package driver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/openkcm/cmk/internal/multitenancy/gmterrors"
)

type (
	// DBFactory defines operations for managing the lifecycle of tenants within a multitenant
	// database architecture. It abstracts tenant-specific operations such as onboarding,
	// offboarding, and managing shared resources.
	DBFactory interface {
		// RegisterModels registers GORM model structs for multitenancy support within a specific database.
		// It prepares models for tenant-specific operations and is idempotent. Returns an error if registration fails.
		RegisterModels(ctx context.Context, db *gorm.DB, models ...TenantTabler) error

		// MigrateSharedModels ensures shared data structures are set up and up-to-date within a specific database,
		// maintaining integrity and compatibility of shared data across tenants. Returns an error if migration fails.
		MigrateSharedModels(ctx context.Context, db *gorm.DB) error

		// MigrateTenantModels prepares and updates data structures for a specific tenant within a specific database,
		// handling onboarding and ongoing schema evolution. Returns an error if setup or migration fails.
		MigrateTenantModels(ctx context.Context, db *gorm.DB, tenantID string) error

		// OffboardTenant cleans up the database for a removed tenant, supporting clean
		// offboarding. Returns an error if the process fails.
		OffboardTenant(ctx context.Context, db *gorm.DB, tenantID string) error

		// UseTenant configures the database for operations specific to a tenant, abstracting
		// database-specific tenant context configuration. Returns a reset function to revert
		// the database context and an error if the operation fails.
		UseTenant(ctx context.Context, db *gorm.DB, tenantID string) (reset func() error, err error)

		// CurrentTenant returns the identifier for the current tenant context within a specific database or an empty string
		// if no context is set.
		CurrentTenant(ctx context.Context, db *gorm.DB) string
	}

	// TenantTabler defines an interface for models within a multi-tenant architecture,
	// extending [schema.Tabler]. Models must define their table name and indicate if they
	// are shared across tenants. Crucial for differentiating between shared and tenant-specific data.
	//
	// Example of a shared model:
	//
	// 	func (User) TableName() string { return "public.users" }
	// 	func (User) IsSharedModel() bool { return true }
	//
	// Example of a tenant-specific model:
	//
	// 	func (Product) TableName() string { return "products" }
	// 	func (Product) IsSharedModel() bool { return false }
	TenantTabler interface {
		schema.Tabler
		// IsSharedModel returns true if the model is shared across tenants, indicating
		// it does not belong to a single tenant.
		IsSharedModel() bool
	}
)

// ErrInvalidMigration is returned when an invalid migration is detected.
var ErrInvalidMigration = errors.New(
	"invalid migration: use MigrateSharedModels or MigrateTenantModels instead of calling AutoMigrate directly",
)

// Errors returned during model registration validation.
var (
	ErrInvalidSharedTableName = errors.New("invalid table name for model labeled as public table")
	ErrInvalidTenantTableName = errors.New("invalid table name for model labeled as tenant table")
)

// PublicSchemaEnvVar is the environment variable that contains the name of the public schema.
const PublicSchemaEnvVar = "GMT_PUBLIC_SCHEMA_NAME"

// PublicSchemaName returns the name of the public schema as defined by the [PublicSchemaEnvVar]
// environment variable, defaulting to "public" if the variable is not set. This schema name is
// used to identify shared models.
func PublicSchemaName() string {
	return cmp.Or(os.Getenv(PublicSchemaEnvVar), "public")
}

// ModelRegistry holds the models registered for multitenancy support, categorizing them into
// shared and tenant-specific models. Not intended for direct use in application code.
type ModelRegistry struct {
	SharedModels []TenantTabler // SharedModels contains the models that are shared across tenants.
	TenantModels []TenantTabler // TenantModels contains the models that are specific to a tenant.
}

// NewModelRegistry creates and initializes a new ModelRegistry with the provided models, categorizing them into
// shared and tenant-specific based on their characteristics. It returns an error if any model fails validation.
func NewModelRegistry(models ...TenantTabler) (*ModelRegistry, error) {
	var (
		registry = &ModelRegistry{
			SharedModels: make([]TenantTabler, 0, len(models)),
			TenantModels: make([]TenantTabler, 0, len(models)),
		}
		errs []error
	)

	for _, model := range models {
		tableName := model.TableName()
		if model.IsSharedModel() {
			if err := validateSharedModel(tableName); err != nil {
				errs = append(errs, err)
				continue
			}
			registry.SharedModels = append(registry.SharedModels, model)
		} else {
			if err := validateTenantModel(tableName); err != nil {
				errs = append(errs, err)
				continue
			}
			registry.TenantModels = append(registry.TenantModels, model)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return registry, nil
}

// splitTableName splits a table name into its constituent parts, typically schema and table name.
func splitTableName(tableName string) []string {
	return strings.Split(tableName, ".")
}

// validateSharedModel checks if a shared model's table name conforms to the expected naming convention,
// which includes the public schema prefix. It returns an error if the validation fails.
func validateSharedModel(tableName string) error {
	parts := splitTableName(tableName)
	public := PublicSchemaName()
	if len(parts) != 2 || parts[0] != public {
		return gmterrors.New(
			fmt.Errorf("%w: should start with '%s.', got '%s'", ErrInvalidSharedTableName, public, tableName))
	}
	return nil
}

// validateTenantModel verifies that a tenant model's table name does not include a schema prefix,
// ensuring it is tenant-specific. It returns an error if the validation fails.
func validateTenantModel(tableName string) error {
	parts := splitTableName(tableName)
	if len(parts) > 1 {
		return gmterrors.New(fmt.Errorf("%w: should not contain a fullstop, got '%s'", ErrInvalidTenantTableName, tableName))
	}
	return nil
}

// ModelsToInterfaces converts a slice of [TenantTabler] models to a slice of any.
func ModelsToInterfaces(models []TenantTabler) []any {
	interfaceModels := make([]any, len(models))
	for i, model := range models {
		interfaceModels[i] = model
	}
	return interfaceModels
}
