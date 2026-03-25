package mapping

import "fmt"

var (
	ErrMappingNotFound = fmt.Errorf("mapping not found")
	ErrMappingAlreadyExists = fmt.Errorf("mapping already exists")
	ErrInvalidExternalID = fmt.Errorf("invalid external ID")
	ErrInvalidType = fmt.Errorf("invalid type")
	ErrInvalidTenantID = fmt.Errorf("invalid tenant ID")
	ErrSystemNotMapped = fmt.Errorf("system is not mapped to tenant")
	ErrOperationFailed = fmt.Errorf("operation failed")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
