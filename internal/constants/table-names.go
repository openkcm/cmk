package constants

const (
	publicTablePreFix = "public."

	CertificateTable      = "certificates"
	EventTable            = "events"
	GroupTable            = "groups"
	ImportparamTable      = "import_params"
	KeyTable              = "keys"
	KeyconfigurationTable = "key_configurations"
	KeystoreTable         = publicTablePreFix + "keystore_pool"
	KeyVersionTable       = "key_versions"
	KeyLabelTable         = "key_labels"
	SystemTable           = "systems"
	SystemPropertyTable   = "systems_properties"
	TagTable              = "tags"
	TenantTable           = publicTablePreFix + "tenants"
	TenantconfigTable     = "tenant_configs"
	WorkflowTable         = "workflows"
	WorkflowApproverTable = "workflow_approvers"
)
