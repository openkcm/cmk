package multitenancy

// TenantModel a basic GoLang struct which includes the following fields: DomainURL, SchemaName.
// It's intended to be embedded into any public model that needs to be scoped to a tenant.
//
// For example:
//
//	type Tenant struct {
//	  multitenancy.TenantModel
//	}
type TenantModel struct {
	// DomainURL is the domain URL of the tenant; same as [net/url.URL.Host].
	DomainURL string `json:"domainURL" mapstructure:"domainURL" gorm:"column:domain_url;uniqueIndex;size:128"`

	// SchemaName is the schema name of the tenant.
	//
	// Field-level permissions are restricted to read and create.
	//
	// The following constraints are applied:
	// 	- unique index
	// 	- size: 63
	//  - check: Not less than 3 characters long
	//nolint:lll // gorm struct tags must be a single line; the DB column constraints are load-bearing
	SchemaName string `json:"schemaName" mapstructure:"schemaName" gorm:"column:schema_name;uniqueIndex;->;<-:create;size:63;check:LENGTH(schema_name) >= 3"`
}
