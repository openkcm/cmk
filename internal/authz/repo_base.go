package authz

import "github.com/openkcm/cmk/internal/constants"

type (
	RepoAction           string
	RepoResourceTypeName string
	RepoResourceType     struct {
		ID      RepoResourceTypeName
		Actions []RepoAction
	}
)

// all resource types which are used in policies
// These are linked to table names, so will require a migration if changed.
// Having this linkage ensures that tables are more coupled to the authz resource identifiers
const (
	RepoResourceTypeCertificate      RepoResourceTypeName = RepoResourceTypeName(constants.CertificateTable)
	RepoResourceTypeEvent            RepoResourceTypeName = RepoResourceTypeName(constants.EventTable)
	RepoResourceTypeGroup            RepoResourceTypeName = RepoResourceTypeName(constants.GroupTable)
	RepoResourceTypeImportparam      RepoResourceTypeName = RepoResourceTypeName(constants.ImportparamTable)
	RepoResourceTypeKey              RepoResourceTypeName = RepoResourceTypeName(constants.KeyTable)
	RepoResourceTypeKeyconfiguration RepoResourceTypeName = RepoResourceTypeName(constants.KeyconfigurationTable)
	RepoResourceTypeKeystore         RepoResourceTypeName = RepoResourceTypeName(constants.KeystoreTable)
	RepoResourceTypeKeyversion       RepoResourceTypeName = RepoResourceTypeName(constants.KeyVersionTable)
	RepoResourceTypeKeyLabel         RepoResourceTypeName = RepoResourceTypeName(constants.KeyLabelTable)
	RepoResourceTypeSystem           RepoResourceTypeName = RepoResourceTypeName(constants.SystemTable)
	RepoResourceTypeSystemProperty   RepoResourceTypeName = RepoResourceTypeName(constants.SystemPropertyTable)
	RepoResourceTypeTag              RepoResourceTypeName = RepoResourceTypeName(constants.TagTable)
	RepoResourceTypeTenant           RepoResourceTypeName = RepoResourceTypeName(constants.TenantTable)
	RepoResourceTypeTenantconfig     RepoResourceTypeName = RepoResourceTypeName(constants.TenantconfigTable)
	RepoResourceTypeWorkflow         RepoResourceTypeName = RepoResourceTypeName(constants.WorkflowTable)
	RepoResourceTypeWorkflowApprover RepoResourceTypeName = RepoResourceTypeName(constants.WorkflowApproverTable)
)

// all actions which are used in policies which can be performed on resource types
const (
	RepoActionList   RepoAction = "list"
	RepoActionFirst  RepoAction = "first"
	RepoActionCount  RepoAction = "count"
	RepoActionCreate RepoAction = "create"
	RepoActionUpdate RepoAction = "update"
	RepoActionDelete RepoAction = "delete"
)

var fullActionList = []RepoAction{
	RepoActionList,
	RepoActionFirst,
	RepoActionCount,
	RepoActionCreate,
	RepoActionUpdate,
	RepoActionDelete,
}

var RepoResourceTypeActions = map[RepoResourceTypeName][]RepoAction{
	RepoResourceTypeCertificate:      fullActionList,
	RepoResourceTypeEvent:            fullActionList,
	RepoResourceTypeGroup:            fullActionList,
	RepoResourceTypeImportparam:      fullActionList,
	RepoResourceTypeKey:              fullActionList,
	RepoResourceTypeKeyconfiguration: fullActionList,
	RepoResourceTypeKeystore:         fullActionList,
	RepoResourceTypeKeyversion:       fullActionList,
	RepoResourceTypeKeyLabel:         fullActionList,
	RepoResourceTypeSystem:           fullActionList,
	RepoResourceTypeSystemProperty:   fullActionList,
	RepoResourceTypeTag:              fullActionList,
	RepoResourceTypeTenant:           fullActionList,
	RepoResourceTypeTenantconfig:     fullActionList,
	RepoResourceTypeWorkflow:         fullActionList,
	RepoResourceTypeWorkflowApprover: fullActionList,
}
