package providers

import "time"

// KeyState represents the state of the key.
type KeyState string

const (
	ENABLED  KeyState = "ENABLED"
	DISABLED KeyState = "DISABLED"
	DELETED  KeyState = "DELETED"
	ERROR    KeyState = "ERROR"
)

// KeyAlgorithm represents the algorithm of the key.
type KeyAlgorithm string

const (
	AES256  KeyAlgorithm = "AES256"
	RSA3072 KeyAlgorithm = "RSA3072"
	RSA4096 KeyAlgorithm = "RSA4096"
)

// KeyVersion represents the version of a key.
type KeyVersion struct {
	ExternalID *string
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
	Version    int
	State      KeyState
}

// Key represents a key.
type Key struct {
	ID          *string
	KeyType     KeyAlgorithm
	Provider    string
	Region      string
	Version     int
	KeyVersions []KeyVersion
}
