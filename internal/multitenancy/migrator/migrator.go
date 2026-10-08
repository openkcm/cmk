// Package migrator provides utilities for database migration management.
package migrator

import (
	"errors"
	"hash/fnv"
	"math/big"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy/driver"
)

const pkgName = "multitenancy/migrator"

type key string

const migratorKey key = pkgName + "/migrator"

type option uint

// Define values for [option].
const (
	DefaultOption option = iota
	MigratorOption
)

// WithOption sets the migration option for the database.
func WithOption(opt option) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Set(string(migratorKey), opt)
	}
}

// OptionFromDB retrieves the migration option from the database.
// If the option is not found or is not of the correct type, it returns [driver.ErrInvalidMigration].
func OptionFromDB(db *gorm.DB) (option, error) {
	o, optFound := db.Get(string(migratorKey))

	if !optFound || o == nil {
		return 0, driver.ErrInvalidMigration
	}

	optVal, ok := o.(option)
	if !ok {
		return 0, driver.ErrInvalidMigration
	}

	switch optVal {
	case DefaultOption, MigratorOption:
		return optVal, nil
	default:
		return 0, driver.ErrInvalidMigration
	}
}

// Errors returned when acquiring or releasing advisory locks.
var (
	ErrExecSQL     = errors.New("locking: failed to execute SQL")
	ErrAcquireLock = errors.New("locking: failed to acquire advisory lock")
	ErrReleaseLock = errors.New("locking: failed to release advisory lock")
)

// GenerateLockKey generates a lock key from a string.
func GenerateLockKey(s string) int64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	if err != nil {
		return 0
	}
	bigInt := new(big.Int).SetUint64(h.Sum64())
	return bigInt.Int64()
}
