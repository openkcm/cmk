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
	InternalTenantCLIRole              InternalRole = "INTERNAL_TENANT_CLI"
	InternalEventReconcilerRole        InternalRole = "INTERNAL_EVENT_RECONCILER"
	InternalTaskProcessingRole         InternalRole = "INTERNAL_TASK_PROCESSING"
	InternalTaskCertRotationRole       InternalRole = "INTERNAL_TASK_CERT_ROTATION"
	InternalTaskWorkflowApproversRole  InternalRole = "INTERNAL_TASK_WORKFLOW_APPROVERS"
	InternalTaskWorkflowCleanupRole    InternalRole = "INTERNAL_TASK_WORKFLOW_CLEANUP"
	InternalTaskWorkflowExpirationRole InternalRole = "INTERNAL_TASK_WORKFLOW_EXPIRATION"
	InternalTaskHYOKSyncRole           InternalRole = "INTERNAL_TASK_HYOK_SYNC"
	InternalTaskKeystorePoolRole       InternalRole = "INTERNAL_TASK_KEYSTORE_POOL"
	InternalTaskSystemRefreshRole      InternalRole = "INTERNAL_TASK_SYSTEM_REFRESH"
	InternalTaskTenantRefreshRole      InternalRole = "INTERNAL_TASK_TENANT_REFRESH"
	InternalTaskSendNotificationRole   InternalRole = "INTERNAL_TASK_SEND_NOTIFICATION"

	AuditorPolicy     PolicyID = "AuditorPolicy"
	KeyAdminPolicy    PolicyID = "KeyAdminPolicy"
	TenantAdminPolicy PolicyID = "TenantAdminPolicy"

	InternalTenantCLIPolicy              PolicyID = "InternalTenantCLI"
	InternalBusinessAuthzPolicy          PolicyID = "InternalBusinessAuthz"
	InternalEventReconcilerPolicy        PolicyID = "InternalEventReconciler"
	InternalTenantProvisioningPolicy     PolicyID = "InternalTenantProvisioning"
	InternalTaskProcessingPolicy         PolicyID = "InternalTaskProcessing"
	InternalTaskCertRotationPolicy       PolicyID = "InternalTaskCertRotation"
	InternalTaskWorkflowApproversPolicy  PolicyID = "InternalTaskWorkflowApprovers"
	InternalTaskHYOKSyncPolicy           PolicyID = "InternalTaskHYOKSync"
	InternalTaskKeystorePoolPolicy       PolicyID = "InternalTaskKeystorePool"
	InternalTaskSystemRefreshPolicy      PolicyID = "InternalTaskSystemRefresh"
	InternalTaskTenantRefreshPolicy      PolicyID = "InternalTaskTenantRefresh"
	InternalTaskWorkflowCleanupPolicy    PolicyID = "InternalTaskWorkflowCleanup"
	InternalTaskWorkflowExpirationPolicy PolicyID = "InternalTaskWorkflowExpiration"
)

type (
	BusinessRole string
	InternalRole string
	PolicyID     string
)
