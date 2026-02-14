package cmkplugincatalog

import (
	"errors"

	"github.com/openkcm/plugin-sdk/api/service/certificateissuer"
	"github.com/openkcm/plugin-sdk/api/service/identitymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keystoremanagement"
	"github.com/openkcm/plugin-sdk/api/service/notification"
	"github.com/openkcm/plugin-sdk/api/service/systeminformation"
	"github.com/zeebo/errs/v2"

	serviceapi "github.com/openkcm/plugin-sdk/api/service"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
)

type ServiceRegistry interface {
	CertificateIssuer() certificateissuer.CertificateIssuer
	Notification() notification.Notification
	SystemInformation() systeminformation.SystemInformation
	IdentityManagement() identitymanagement.IdentityManagement

	KeystoreManagements() map[string]keystoremanagement.KeystoreManagement
	KeystoreManagementList() []keystoremanagement.KeystoreManagement
	KeyManagements() map[string]keymanagement.KeyManagement
	KeyManagementList() []keymanagement.KeyManagement
}

var _ ServiceRegistry = (*Registry)(nil)

type Registry struct {
	registry              serviceapi.Registry
	plugincatalog.Catalog //nolint:embeddedstructfieldcheck
}

func NewPluginCatalog(clg *plugincatalog.Catalog) *Registry {
	return &Registry{
		registry: plugincatalog.WrapAsPluginRepository(clg),

		Catalog: *clg,
	}
}

func (p *Registry) Validate() error {
	groupError := &errs.Group{}

	if _, ok := p.registry.Notification(); !ok {
		//nolint: err113
		err := errors.New("notification plugin is mandatory")
		groupError.Append(err)
	}

	if _, ok := p.registry.SystemInformation(); !ok {
		//nolint: err113
		err := errors.New("system information plugin is mandatory")
		groupError.Append(err)
	}

	return groupError.Err()
}

func (p *Registry) CertificateIssuer() certificateissuer.CertificateIssuer {
	instance, _ := p.registry.CertificateIssuer()
	return instance
}

func (p *Registry) Notification() notification.Notification {
	instance, _ := p.registry.Notification()
	return instance
}

func (p *Registry) SystemInformation() systeminformation.SystemInformation {
	instance, _ := p.registry.SystemInformation()
	return instance
}

func (p *Registry) IdentityManagement() identitymanagement.IdentityManagement {
	instance, _ := p.registry.IdentityManagement()
	return instance
}

func (p *Registry) KeystoreManagements() map[string]keystoremanagement.KeystoreManagement {
	instance, _ := p.registry.KeystoreManagements()
	return instance
}

func (p *Registry) KeystoreManagementList() []keystoremanagement.KeystoreManagement {
	instance, _ := p.registry.KeystoreManagementList()
	return instance
}

func (p *Registry) KeyManagements() map[string]keymanagement.KeyManagement {
	instance, _ := p.registry.KeyManagements()
	return instance
}

func (p *Registry) KeyManagementList() []keymanagement.KeyManagement {
	instance, _ := p.registry.KeyManagementList()
	return instance
}
