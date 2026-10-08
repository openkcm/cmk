// Package enums provides generic SQL Valuer/Scanner helpers for string-based
// enum types whose set of valid values is exposed via a Valid() method.
package enums

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

var ErrUnexpectedScanType = errors.New("unexpected scan type")

// Validator is satisfied by string-based enum types that report membership
// in their valid set via Valid().
type Validator interface {
	~string
	Valid() bool
}

// Value implements driver.Valuer. Empty values become SQL NULL; non-empty
// values must satisfy v.Valid().
func Value[T Validator](v T, invalidErr error) (driver.Value, error) {
	if v == "" {
		return nil, nil //nolint:nilnil
	}
	if !v.Valid() {
		return nil, fmt.Errorf("%w: %q", invalidErr, string(v))
	}
	return string(v), nil
}

// Scan implements sql.Scanner. nil clears to the zero value; string and
// []byte are accepted and validated via Valid().
func Scan[T Validator](src any, out *T, invalidErr error) error {
	if src == nil {
		var zero T
		*out = zero
		return nil
	}
	var sv string
	switch v := src.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, src)
	}
	v := T(sv)
	if !v.Valid() {
		return fmt.Errorf("%w: %q", invalidErr, sv)
	}
	*out = v
	return nil
}
