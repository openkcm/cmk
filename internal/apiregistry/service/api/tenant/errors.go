package tenant

import "fmt"

var (
	ErrTenantNotFound          = fmt.Errorf("tenant not found")
	ErrTenantAlreadyExists     = fmt.Errorf("tenant already exists")
	ErrInvalidTenantID         = fmt.Errorf("invalid tenant ID")
	ErrInvalidRegion           = fmt.Errorf("invalid region")
	ErrInvalidOwnerID          = fmt.Errorf("invalid owner ID")
	ErrInvalidOwnerType        = fmt.Errorf("invalid owner type")
	ErrInvalidTenantName       = fmt.Errorf("invalid tenant name")
	ErrInvalidLimit            = fmt.Errorf("invalid limit: must be between 1 and 1000")
	ErrTenantAlreadyBlocked    = fmt.Errorf("tenant is already blocked")
	ErrTenantNotBlocked        = fmt.Errorf("tenant is not blocked")
	ErrTenantAlreadyTerminated = fmt.Errorf("tenant is already terminated")
	ErrInvalidTenantStatus     = fmt.Errorf("invalid tenant status for this operation")
	ErrInvalidTenantRole       = fmt.Errorf("invalid tenant role")
	ErrInvalidLabels           = fmt.Errorf("invalid labels")
	ErrInvalidLabelKeys        = fmt.Errorf("invalid label keys")
	ErrInvalidUserGroups       = fmt.Errorf("invalid user groups")
	ErrOperationFailed         = fmt.Errorf("operation failed")
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
