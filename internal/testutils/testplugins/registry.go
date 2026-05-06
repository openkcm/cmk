package testplugins

import (
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/certificateissuer"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
	"github.com/openkcm/cmk/internal/pluginregistry/service/wrapper/system_information"
)

// Registry is a test implementation of serviceapi.Registry that holds native
// Go service implementations, bypassing the plugin-sdk catalog and gRPC layer.
type Registry struct {
	certificateIssuer   certificateissuer.CertificateIssuer
	identityManagement  identitymanagement.IdentityManagement
	keystoreManagements map[string]keystoremanagement.KeystoreManagement
	keyManagements      map[string]keymanagement.KeyManagement
	notificationSvc     notification.Notification
	systemInformation   systeminformation.SystemInformation
}

// Compile-time assertion that Registry implements serviceapi.Registry.
var _ serviceapi.Registry = (*Registry)(nil)

// RegistryOption configures a Registry.
type RegistryOption func(*Registry)

// WithCertificateIssuer sets the CertificateIssuer service.
func WithCertificateIssuer(svc certificateissuer.CertificateIssuer) RegistryOption {
	return func(r *Registry) { r.certificateIssuer = svc }
}

// WithIdentityManagement sets the IdentityManagement service.
func WithIdentityManagement(svc identitymanagement.IdentityManagement) RegistryOption {
	return func(r *Registry) { r.identityManagement = svc }
}

// WithKeystoreManagement adds a KeystoreManagement service under the given key.
func WithKeystoreManagement(key string, svc keystoremanagement.KeystoreManagement) RegistryOption {
	return func(r *Registry) {
		if r.keystoreManagements == nil {
			r.keystoreManagements = make(map[string]keystoremanagement.KeystoreManagement)
		}
		r.keystoreManagements[key] = svc
	}
}

// WithKeyManagement adds a KeyManagement service under the given key.
func WithKeyManagement(key string, svc keymanagement.KeyManagement) RegistryOption {
	return func(r *Registry) {
		if r.keyManagements == nil {
			r.keyManagements = make(map[string]keymanagement.KeyManagement)
		}
		r.keyManagements[key] = svc
	}
}

// WithNotification sets the Notification service.
func WithNotification(svc notification.Notification) RegistryOption {
	return func(r *Registry) { r.notificationSvc = svc }
}

// WithSystemInformation sets the SystemInformation service.
func WithSystemInformation(svc systeminformation.SystemInformation) RegistryOption {
	return func(r *Registry) { r.systemInformation = svc }
}

// WithNoSystemInformation clears the SystemInformation service so that
// SystemInformation() returns system_information.ErrNotConfigured.
func WithNoSystemInformation() RegistryOption {
	return func(r *Registry) { r.systemInformation = nil }
}

// NewRegistry creates a Registry prepopulated with default test service implementations.
// Individual services can be overridden via RegistryOption.
func NewRegistry(opts ...RegistryOption) *Registry {
	r := &Registry{
		certificateIssuer:   NewTestCertificateIssuer(),
		identityManagement:  NewTestIdentityManagement(),
		notificationSvc:     NewTestNotification(),
		systemInformation:   NewTestSystemInformation(),
		keystoreManagements: map[string]keystoremanagement.KeystoreManagement{Name: NewTestKeystoreManagement()},
		keyManagements:      map[string]keymanagement.KeyManagement{Name: NewTestKeyManagement(true, true)},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Registry) Close() error { return nil }

func (r *Registry) CertificateIssuer() (certificateissuer.CertificateIssuer, error) {
	return r.certificateIssuer, nil
}

func (r *Registry) Notification() (notification.Notification, error) {
	return r.notificationSvc, nil
}

func (r *Registry) SystemInformation() (systeminformation.SystemInformation, error) {
	if r.systemInformation == nil {
		return nil, system_information.ErrNotConfigured
	}
	return r.systemInformation, nil
}

func (r *Registry) IdentityManagement() (identitymanagement.IdentityManagement, error) {
	return r.identityManagement, nil
}

func (r *Registry) KeystoreManagements() (map[string]keystoremanagement.KeystoreManagement, error) {
	return r.keystoreManagements, nil
}

func (r *Registry) KeystoreManagementList() ([]keystoremanagement.KeystoreManagement, error) {
	list := make([]keystoremanagement.KeystoreManagement, 0, len(r.keystoreManagements))
	for _, svc := range r.keystoreManagements {
		list = append(list, svc)
	}
	return list, nil
}

func (r *Registry) KeyManagements() (map[string]keymanagement.KeyManagement, error) {
	return r.keyManagements, nil
}

func (r *Registry) KeyManagementList() ([]keymanagement.KeyManagement, error) {
	list := make([]keymanagement.KeyManagement, 0, len(r.keyManagements))
	for _, svc := range r.keyManagements {
		list = append(list, svc)
	}
	return list, nil
}
