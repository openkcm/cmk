package serviceapi

import (
	"io"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
)

// Registry defines the central contract for accessing and managing system services.
// It embeds io.Closer to facilitate the graceful shutdown of all active subsystems.
type Registry interface {
	io.Closer

	// CertificateIssuer returns the active CertificateIssuer service.
	CertificateIssuer() (certificateissuer.CertificateIssuer, error)

	// Notification returns the active Notification service.
	Notification() (notification.Notification, error)

	// SystemInformation returns the active SystemInformation service.
	SystemInformation() (systeminformation.SystemInformation, error)

	// IdentityManagement returns the active IdentityManagement service.
	IdentityManagement() (identitymanagement.IdentityManagement, error)

	// KeystoreManagements returns a map of all available KeystoreManagement services,
	// typically keyed by their unique configuration name or provider ID (e.g., "aws-kms").
	KeystoreManagements() (map[string]keystoremanagement.KeystoreManagement, error)

	// KeystoreManagementList returns a slice of all available KeystoreManagement services.
	// This is optimized for scenarios where ordered iteration is preferred over key lookup.
	KeystoreManagementList() ([]keystoremanagement.KeystoreManagement, error)

	// KeyManagements returns a map of all available KeyManagement services,
	// typically keyed by their unique configuration name or provider ID.
	KeyManagements() (map[string]keymanagement.KeyManagement, error)

	// KeyManagementList returns a slice of all available KeyManagement services.
	// This is optimized for scenarios where ordered iteration is preferred over key lookup.
	KeyManagementList() ([]keymanagement.KeyManagement, error)
}
