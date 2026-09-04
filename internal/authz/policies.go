package authz

import "github.com/openkcm/cmk/internal/constants"

type (
	Resource[
		Type APIResourceType | RepoResourceType,
		Action APIAction | RepoAction,
	] struct {
		Type    Type
		Actions []Action
	}

	Policy[
		ResourceType APIResourceType | RepoResourceType,
		Action APIAction | RepoAction,
	] struct {
		ID            constants.PolicyID
		ResourceTypes []Resource[ResourceType, Action]
	}

	RolePolicies[
		Role constants.BusinessRole | constants.InternalRole,
		Resource APIResourceType | RepoResourceType,
		Action APIAction | RepoAction,
	] map[Role][]Policy[Resource, Action]

	RepoAction       string
	RepoResourceType string

	APIAction       string
	APIResourceType string
)

// These are linked to table names, so will require a migration if changed.
// Having this linkage ensures that tables are more coupled to the authz resource identifiers
const (
	RepoResourceTypeCertificate      RepoResourceType = RepoResourceType(constants.CertificateTable)
	RepoResourceTypeEvent            RepoResourceType = RepoResourceType(constants.EventTable)
	RepoResourceTypeGroup            RepoResourceType = RepoResourceType(constants.GroupTable)
	RepoResourceTypeImportparam      RepoResourceType = RepoResourceType(constants.ImportparamTable)
	RepoResourceTypeKey              RepoResourceType = RepoResourceType(constants.KeyTable)
	RepoResourceTypeKeyconfiguration RepoResourceType = RepoResourceType(constants.KeyconfigurationTable)
	RepoResourceTypeKeystore         RepoResourceType = RepoResourceType(constants.KeystoreTable)
	RepoResourceTypeKeyversion       RepoResourceType = RepoResourceType(constants.KeyVersionTable)
	RepoResourceTypeKeyLabel         RepoResourceType = RepoResourceType(constants.KeyLabelTable)
	RepoResourceTypeResourceLabel    RepoResourceType = RepoResourceType(constants.ResourceLabelTable)
	RepoResourceTypeSystem           RepoResourceType = RepoResourceType(constants.SystemTable)
	RepoResourceTypeSystemProperty   RepoResourceType = RepoResourceType(constants.SystemPropertyTable)
	RepoResourceTypeTag              RepoResourceType = RepoResourceType(constants.TagTable)
	RepoResourceTypeTenant           RepoResourceType = RepoResourceType(constants.TenantTable)
	RepoResourceTypeTenantconfig     RepoResourceType = RepoResourceType(constants.TenantconfigTable)
	RepoResourceTypeWorkflow         RepoResourceType = RepoResourceType(constants.WorkflowTable)
	RepoResourceTypeWorkflowApprover RepoResourceType = RepoResourceType(constants.WorkflowApproverTable)

	RepoActionList   RepoAction = "list"
	RepoActionFirst  RepoAction = "first"
	RepoActionCount  RepoAction = "count"
	RepoActionCreate RepoAction = "create"
	RepoActionUpdate RepoAction = "update"
	RepoActionDelete RepoAction = "delete"

	APIResourceTypeKeyConfiguration APIResourceType = "KeyConfiguration"
	APIResourceTypeKey              APIResourceType = "Key"
	APIResourceTypeSystem           APIResourceType = "System"
	APIResourceTypeWorkFlow         APIResourceType = "Workflow"
	APIResourceTypeUserGroup        APIResourceType = "UserGroup"
	APIResourceTypeTenant           APIResourceType = "Tenant"
	APIResourceTypeTenantSettings   APIResourceType = "TenantSettings"
	APIResourceTypeEvent            APIResourceType = "Event"
	APIResourceTypeImportParams     APIResourceType = "ImportParams"
	APIResourceTypeKeyStoreConfig   APIResourceType = "KeyStoreConfig"

	APIActionRead             APIAction = "read"
	APIActionCreate           APIAction = "create"
	APIActionUpdate           APIAction = "update"
	APIActionDelete           APIAction = "delete"
	APIActionKeyRotate        APIAction = "KeyRotate"
	APIActionSystemModifyLink APIAction = "ModifySystemLink"
)

var repoActionList = []RepoAction{
	RepoActionList,
	RepoActionFirst,
	RepoActionCount,
	RepoActionCreate,
	RepoActionUpdate,
	RepoActionDelete,
}

var RepoResourceTypeActions = map[RepoResourceType][]RepoAction{
	RepoResourceTypeCertificate:      repoActionList,
	RepoResourceTypeEvent:            repoActionList,
	RepoResourceTypeGroup:            repoActionList,
	RepoResourceTypeImportparam:      repoActionList,
	RepoResourceTypeKey:              repoActionList,
	RepoResourceTypeKeyconfiguration: repoActionList,
	RepoResourceTypeKeystore:         repoActionList,
	RepoResourceTypeKeyversion:       repoActionList,
	RepoResourceTypeKeyLabel:         repoActionList,
	RepoResourceTypeResourceLabel:    repoActionList,
	RepoResourceTypeSystem:           repoActionList,
	RepoResourceTypeSystemProperty:   repoActionList,
	RepoResourceTypeTag:              repoActionList,
	RepoResourceTypeTenant:           repoActionList,
	RepoResourceTypeTenantconfig:     repoActionList,
	RepoResourceTypeWorkflow:         repoActionList,
	RepoResourceTypeWorkflowApprover: repoActionList,
}

var APIResourceTypeActions = map[APIResourceType][]APIAction{
	APIResourceTypeKeyConfiguration: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeKey: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
		APIActionKeyRotate,
	},
	APIResourceTypeSystem: {
		APIActionRead,
		APIActionSystemModifyLink,
	},
	APIResourceTypeWorkFlow: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeTenantSettings: {
		APIActionRead,
		APIActionUpdate,
	},
	APIResourceTypeUserGroup: {
		APIActionRead,
		APIActionCreate,
		APIActionDelete,
		APIActionUpdate,
	},
	APIResourceTypeTenant: {
		APIActionRead,
		APIActionUpdate,
	},
}
