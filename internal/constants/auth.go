package constants

const (
	//nolint:gosec
	AuthTypeSecret      = "AUTH_TYPE_SECRET"
	AuthTypeCertificate = "AUTH_TYPE_CERTIFICATE"

	TenantAdminGroup   string = "TenantAdministrator"
	TenantAuditorGroup string = "TenantAuditor"

	KeyAdminRole      Role = "KEY_ADMINISTRATOR"
	TenantAdminRole   Role = "TENANT_ADMINISTRATOR"
	TenantAuditorRole Role = "TENANT_AUDITOR"
)

type Role string
