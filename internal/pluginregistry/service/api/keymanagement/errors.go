package keymanagement

import "errors"

var (
	ErrProviderAuthenticationFailed = errors.New("failed to authenticate with the keystore provider")
	ErrHYOKKeyNotFound              = errors.New("HYOK provider key not found")
	ErrGenericGetKeyError           = errors.New("failed to get key")
	ErrImportKeyMaterialFailed      = errors.New("invalid or incorrectly wrapped key material")
)

// ProviderAuthError wraps ErrProviderAuthenticationFailed and carries the provider-specific
// reason string (e.g. "DENIED_BY_POLICY") from the gRPC ErrorInfo detail.
type ProviderAuthError struct {
	Reason string
}

func (e *ProviderAuthError) Error() string { return e.Reason }
func (e *ProviderAuthError) Is(target error) bool {
	return target == ErrProviderAuthenticationFailed
}
func (e *ProviderAuthError) Unwrap() error { return ErrProviderAuthenticationFailed }
