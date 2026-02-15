package cmkplugincatalog

import (
	"github.com/openkcm/plugin-sdk/api/service/certificateissuer"
	"github.com/openkcm/plugin-sdk/api/service/identitymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keystoremanagement"
	"github.com/openkcm/plugin-sdk/api/service/notification"
	"github.com/openkcm/plugin-sdk/api/service/systeminformation"
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	serviceapi "github.com/openkcm/plugin-sdk/api/service"
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
	serviceapi.Registry
	catalog.Catalog
}

func NewPluginCatalog(clg *catalog.Catalog) *Registry {
	return &Registry{
		Registry: catalog.WrapAsPluginRepository(clg),
		Catalog:  *clg,
	}
}

func (p *Registry) Close() error {
	return p.Registry.Close()
}

func (p *Registry) CertificateIssuer() certificateissuer.CertificateIssuer {
	instance, _ := p.Registry.CertificateIssuer()
	return instance
}

func (p *Registry) Notification() notification.Notification {
	instance, _ := p.Registry.Notification()
	return instance
}

func (p *Registry) SystemInformation() systeminformation.SystemInformation {
	instance, _ := p.Registry.SystemInformation()
	return instance
}

func (p *Registry) IdentityManagement() identitymanagement.IdentityManagement {
	instance, _ := p.Registry.IdentityManagement()
	return instance
}

func (p *Registry) KeystoreManagements() map[string]keystoremanagement.KeystoreManagement {
	instance, _ := p.Registry.KeystoreManagements()
	return instance
}

func (p *Registry) KeystoreManagementList() []keystoremanagement.KeystoreManagement {
	instance, _ := p.Registry.KeystoreManagementList()
	return instance
}

func (p *Registry) KeyManagements() map[string]keymanagement.KeyManagement {
	instance, _ := p.Registry.KeyManagements()
	return instance
}

func (p *Registry) KeyManagementList() []keymanagement.KeyManagement {
	instance, _ := p.Registry.KeyManagementList()
	return instance
}
