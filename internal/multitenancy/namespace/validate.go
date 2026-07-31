// Package namespace provides utilities for validating tenant names in a
// consistent manner across different database systems.
//
// Namespace is a term used to refer to a tenant; it is a unique identifier
// for a tenant and, for PostgreSQL, is equivalent to the schema name.
package namespace

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/openkcm/cmk/internal/multitenancy/gmterrors"
)

// namespaceRegexStr is the regular expression pattern for a valid schema name.
//
// Examples of valid schema names:
//   - "domain1"
//   - "test_domain"
//   - "test123"
//   - "_domain"
const namespaceRegexStr = `^[_a-zA-Z][_a-zA-Z0-9]{2,}$`

var namespaceRegex = regexp.MustCompile(namespaceRegexStr)

// Tenant name validation errors.
var (
	ErrInvalidPattern = errors.New(
		"tenant name must start with an underscore or a letter, followed by at least two " +
			"characters that can be underscores, letters, or numbers",
	)
	ErrReservedPrefix = errors.New(
		"tenant name must not start with 'pg_' as it is reserved for system schemas in PostgreSQL",
	)
)

// checkPattern validates if the tenant name matches the required pattern.
func checkPattern(tenantID string) error {
	if !namespaceRegex.MatchString(tenantID) {
		return fmt.Errorf("%w: got '%s' (pattern %q)", ErrInvalidPattern, tenantID, namespaceRegexStr)
	}
	return nil
}

// checkPrefix validates if the tenant name starts with a reserved prefix.
func checkPrefix(tenantID string) error {
	if strings.HasPrefix(tenantID, "pg_") {
		return fmt.Errorf("%w: got '%s'", ErrReservedPrefix, tenantID)
	}
	return nil
}

// Validate validates the tenant name.
func Validate(tenantID string) error {
	if err := checkPattern(tenantID); err != nil {
		return gmterrors.New(err)
	}
	if err := checkPrefix(tenantID); err != nil {
		return gmterrors.New(err)
	}
	return nil
}
