package authz

type APIMethod string

// AllowListByAPI is a map acting like a set with O(1) search. As long as the endpoint exists it's valid
var AllowListByAPI = map[string]struct{}{
	"GET /tenants":            {},
	"GET /tenants/{tenantID}": {},
	"GET /userInfo":           {},
	"GET /swagger":            {},
}

type Restricted struct {
	APIResourceTypeName APIResourceType
	APIAction           APIAction
}

// RestrictionsByAPI maps "METHOD path" to authorization requirements
var RestrictionsByAPI = map[string]Restricted{
	// Keys endpoints
	"GET /keys": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"POST /keys": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	"GET /keys/{keyID}": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"PATCH /keys/{keyID}": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionUpdate,
	},
	"DELETE /keys/{keyID}": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionDelete,
	},
	"GET /keys/{keyID}/importParams": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"POST /keys/{keyID}/importKeyMaterial": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionUpdate,
	},
	"GET /keys/{keyID}/versions": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"POST /keys/{keyID}/versions": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	"GET /keys/{keyID}/versions/{version}": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"GET /key/{keyID}/labels": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	"POST /key/{keyID}/labels": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	"DELETE /key/{keyID}/label/{labelName}": {
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionDelete,
	},

	// Key Configurations endpoints
	"GET /keyConfigurations": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	"POST /keyConfigurations": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionCreate,
	},
	"GET /keyConfigurations/{keyConfigurationID}": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	"PATCH /keyConfigurations/{keyConfigurationID}": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionUpdate,
	},
	"DELETE /keyConfigurations/{keyConfigurationID}": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionDelete,
	},
	"GET /keyConfigurations/{keyConfigurationID}/tags": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	"PUT /keyConfigurations/{keyConfigurationID}/tags": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionUpdate,
	},
	"GET /keyConfigurations/{keyConfigurationID}/certificates": {
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},

	// Systems endpoints
	"GET /systems": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionRead,
	},
	"GET /systems/filterOptions": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionRead,
	},
	"GET /systems/{systemID}": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionRead,
	},
	"PATCH /systems/{systemID}/link": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	"DELETE /systems/{systemID}/link": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	"POST /systems/{systemID}/recoveryActions": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	"GET /systems/{systemID}/recoveryActions": {
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},

	// Workflows endpoints
	"POST /workflows": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionCreate,
	},
	"GET /workflows": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	"POST /workflows/check": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionUpdate,
	},
	"GET /workflows/{workflowID}": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	"GET /workflows/{workflowID}/approvers": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	"POST /workflows/{workflowID}/approvers": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionCreate,
	},
	"POST /workflows/{workflowID}/state": {
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionUpdate,
	},

	// Groups endpoints
	"GET /groups": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	"POST /groups": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionCreate,
	},
	"POST /groups/iamCheck": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	"GET /groups/{groupID}": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	"PATCH /groups/{groupID}": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionUpdate,
	},
	"DELETE /groups/{groupID}": {
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionDelete,
	},

	// Tenant endpoints
	"GET /tenantConfigurations/keystores": {
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionRead,
	},
	"GET /tenantConfigurations/workflow": {
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionRead,
	},
	"PATCH /tenantConfigurations/workflow": {
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionUpdate,
	},
	"GET /tenantInfo": {
		APIResourceTypeName: APIResourceTypeTenant,
		APIAction:           APIActionRead,
	},
}
