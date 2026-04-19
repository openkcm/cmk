package constants

const (
	//nolint:gosec
	AuthTypeSecret      = "AUTH_TYPE_SECRET"
	AuthTypeCertificate = "AUTH_TYPE_CERTIFICATE"

	TenantAdminGroup   string = "TenantAdministrator"
	TenantAuditorGroup string = "TenantAuditor"

	KeyAdminRole      BusinessRole = "KEY_ADMINISTRATOR"
	TenantAdminRole   BusinessRole = "TENANT_ADMINISTRATOR"
	TenantAuditorRole BusinessRole = "TENANT_AUDITOR"

	InternalRequestingHandlingRole     InternalRole = "INTERNAL_REQUEST_HANDLING"
	InternalBusinessAuthzRole          InternalRole = "INTERNAL_BUSINESS_AUTH"
	InternalTenantProvisioningRole     InternalRole = "INTERNAL_TENANT_PROVISIONING"
	InternalTaskProcessingRole         InternalRole = "INTERNAL_TASK_PROCESSING"
	InternalTaskCertRotationRole       InternalRole = "INTERNAL_TASK_CERT_ROTATION"
	InternalTaskWorkflowApproversRole  InternalRole = "INTERNAL_TASK_WORKFLOW_APPROVERS"
	InternalTaskWorkflowCleanupRole    InternalRole = "INTERNAL_TASK_WORKFLOW_CLEANUP"
	InternalTaskWorkflowExpirationRole InternalRole = "INTERNAL_TASK_WORKFLOW_EXPIRATION"
	InternalTaskHYOKSyncRole           InternalRole = "INTERNAL_TASK_HYOK_SYNC"
	InternalTaskKeystorePoolRole       InternalRole = "INTERNAL_TASK_KEYSTORE_POOL"
	InternalTaskSystemRefreshRole      InternalRole = "INTERNAL_TASK_SYSTEM_REFRESH"
	InternalTaskTenantRefreshRole      InternalRole = "INTERNAL_TASK_TENANT_REFRESH"
)

type BusinessRole string
type InternalRole string
