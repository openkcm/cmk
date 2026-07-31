package postgres

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrEmptySchemaName is returned when SetSearchPath is called with an empty schema name.
var ErrEmptySchemaName = errors.New("schema name is empty")

// SetSearchPath sets the search path for the given database connection to the specified schema name.
// It returns a function that can be used to reset the search path to the default value.
//
// This function does not perform any validation on the schemaName parameter. It is the
// responsibility of the caller to ensure that the schemaName has been sanitized to avoid SQL
// injection vulnerabilities (the value is quoted via the dialect's identifier quoter).
//
// Technically safe for concurrent use by multiple goroutines, but should not be used concurrently
// w.r.t. ensuring data integrity and schema isolation. Use a separate database connection or
// transaction for each goroutine that requires a different search path.
//
//nolint:nonamedreturns // named returns document the (reset, err) contract callers defer on
func SetSearchPath(tx *gorm.DB, schemaName string) (reset func() error, err error) {
	tx = tx.Session(&gorm.Session{})
	if schemaName == "" {
		_ = tx.AddError(ErrEmptySchemaName)
		return nil, ErrEmptySchemaName
	}
	sqlstr := quoteRawSQLForTenant(tx, "SET search_path TO ", schemaName)
	if execErr := tx.Exec(sqlstr).Error; execErr != nil {
		err = fmt.Errorf("failed to set search path %q: %w", schemaName, execErr)
		_ = tx.AddError(err)
		return nil, err
	}
	reset = func() error { return tx.Exec("SET search_path TO public").Error }
	return reset, nil
}

// CurrentSearchPath returns the current search path for the given database connection.
func CurrentSearchPath(tx *gorm.DB) string {
	tx = tx.Session(&gorm.Session{})
	var searchPath string
	_ = tx.Raw("SHOW search_path").Scan(&searchPath)
	if searchPath == `"$user", public` {
		return "public"
	}
	return searchPath
}
