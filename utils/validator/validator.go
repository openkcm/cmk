package validator

import (
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrValidator = errors.New("validation error")
)

// ValidateUUID checks if the provided ID is a valid UUID.
func ValidateUUID(id string) error {
	_, err := uuid.Parse(id)
	if err != nil {
		return errs.Wrap(ErrValidator, err)
	}

	return nil
}
