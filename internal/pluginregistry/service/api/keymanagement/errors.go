package keymanagement

import "errors"

var (
	ErrProviderAuthenticationFailed = errors.New("failed to authenticate with the keystore provider")
	ErrHYOKKeyNotFound              = errors.New("HYOK provider key not found")
)
