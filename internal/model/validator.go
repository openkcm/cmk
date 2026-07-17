package model

import "errors"

// ErrValidation is the shared parent of all model-level validation errors.
// Every "invalid X" validation sentinel in this package wraps it, so callers can
// classify any validation failure with errors.Is(err, ErrValidation) without
// enumerating each specific error. Such failures are permanent: the input will
// never become valid on retry.
var ErrValidation = errors.New("validation failed")

// Validator defines the methods for validation.
type Validator interface {
	Validate() error
}

// ValidateAll goes through the given validators and calls their Validate method.
// It stops and returns at the first error encountered, if any. If all validate successfully, it returns nil.
func ValidateAll(v ...Validator) error {
	for i := range v {
		err := v[i].Validate()
		if err != nil {
			return err
		}
	}

	return nil
}
