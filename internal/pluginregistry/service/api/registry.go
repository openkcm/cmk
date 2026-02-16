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
	// The boolean returns false if the service is not configured or available.
	CertificateIssuer() (certificateissuer.CertificateIssuer, bool)

	// Notification returns the active Notification service.
	// The boolean returns false if the service is not configured or available.
	Notification() (notification.Notification, bool)

	// SystemInformation returns the active SystemInformation service.
	// The boolean returns false if the service is not configured or available.
	SystemInformation() (systeminformation.SystemInformation, bool)

	// IdentityManagement returns the active IdentityManagement service.
	// The boolean returns false if the service is not configured or available.
	IdentityManagement() (identitymanagement.IdentityManagement, bool)

	// KeystoreManagements returns a map of all available KeystoreManagement services,
	// typically keyed by their unique configuration name or provider ID (e.g., "aws-kms").
	// The boolean returns false if no keystore services are loaded.
	KeystoreManagements() (map[string]keystoremanagement.KeystoreManagement, bool)

	// KeystoreManagementList returns a slice of all available KeystoreManagement services.
	// This is optimized for scenarios where ordered iteration is preferred over key lookup.
	// The boolean returns false if no keystore services are loaded.
	KeystoreManagementList() ([]keystoremanagement.KeystoreManagement, bool)

	// KeyManagements returns a map of all available KeyManagement services,
	// typically keyed by their unique configuration name or provider ID.
	// The boolean returns false if no key management services are loaded.
	KeyManagements() (map[string]keymanagement.KeyManagement, bool)

	// KeyManagementList returns a slice of all available KeyManagement services.
	// This is optimized for scenarios where ordered iteration is preferred over key lookup.
	// The boolean returns false if no key management services are loaded.
	KeyManagementList() ([]keymanagement.KeyManagement, bool)
}
