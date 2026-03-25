package authz

type APIMethod string

const (
	APIMethodGet    APIMethod = "GET"
	APIMethodPost   APIMethod = "POST"
	APIMethodPut    APIMethod = "PUT"
	APIMethodDelete APIMethod = "DELETE"
	APIMethodPatch  APIMethod = "PATCH"
)

type Allowed struct {
	APIPath   string
	APIMethod APIMethod
}

var allowList = []Allowed{
	{
		APIPath:   "/tenants",
		APIMethod: APIMethodGet,
	},
	{
		APIPath:   "/tenants/{tenantID}",
		APIMethod: APIMethodGet,
	},
	{
		APIPath:   "/userInfo",
		APIMethod: APIMethodGet,
	},
}

type Restricted struct {
	APIPath             string
	APIMethod           APIMethod
	APIResourceTypeName APIResourceTypeName
	APIAction           APIAction
}

// Define all restrictions once
var allRestrictions = []Restricted{
	// Keys endpoints
	{
		APIPath:             "/keys",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keys",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/keys/{keyID}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keys/{keyID}",
		APIMethod:           APIMethodPatch,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/keys/{keyID}",
		APIMethod:           APIMethodDelete,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionDelete,
	},
	{
		APIPath:             "/keys/{keyID}/importParams",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keys/{keyID}/importKeyMaterial",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/keys/{keyID}/versions",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keys/{keyID}/versions",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/keys/{keyID}/versions/{version}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/key/{keyID}/labels",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/key/{keyID}/labels",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/key/{keyID}/label/{labelName}",
		APIMethod:           APIMethodDelete,
		APIResourceTypeName: APIResourceTypeKey,
		APIAction:           APIActionDelete,
	},

	// Key Configurations endpoints
	{
		APIPath:             "/keyConfigurations",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keyConfigurations",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}",
		APIMethod:           APIMethodPatch,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}",
		APIMethod:           APIMethodDelete,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionDelete,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}/tags",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}/tags",
		APIMethod:           APIMethodPut,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/keyConfigurations/{keyConfigurationID}/certificates",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeKeyConfiguration,
		APIAction:           APIActionRead,
	},

	// Systems endpoints
	{
		APIPath:             "/systems",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/systems/{systemID}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/systems/{systemID}/link",
		APIMethod:           APIMethodPatch,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	{
		APIPath:             "/systems/{systemID}/link",
		APIMethod:           APIMethodDelete,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	{
		APIPath:             "/systems/{systemID}/recoveryActions",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},
	{
		APIPath:             "/systems/{systemID}/recoveryActions",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeSystem,
		APIAction:           APIActionSystemModifyLink,
	},

	// Workflows endpoints
	{
		APIPath:             "/workflows",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/workflows",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/workflows/check",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/workflows/{workflowID}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/workflows/{workflowID}/approvers",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/workflows/{workflowID}/approvers",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/workflows/{workflowID}/state",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeWorkFlow,
		APIAction:           APIActionUpdate,
	},

	// Groups endpoints
	{
		APIPath:             "/groups",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/groups",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionCreate,
	},
	{
		APIPath:             "/groups/iamCheck",
		APIMethod:           APIMethodPost,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/groups/{groupID}",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/groups/{groupID}",
		APIMethod:           APIMethodPatch,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/groups/{groupID}",
		APIMethod:           APIMethodDelete,
		APIResourceTypeName: APIResourceTypeUserGroup,
		APIAction:           APIActionDelete,
	},

	// Tenant endpoints
	{
		APIPath:             "/tenantConfigurations/keystores",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/tenantConfigurations/workflow",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionRead,
	},
	{
		APIPath:             "/tenantConfigurations/workflow",
		APIMethod:           APIMethodPatch,
		APIResourceTypeName: APIResourceTypeTenantSettings,
		APIAction:           APIActionUpdate,
	},
	{
		APIPath:             "/tenantInfo",
		APIMethod:           APIMethodGet,
		APIResourceTypeName: APIResourceTypeTenant,
		APIAction:           APIActionRead,
	},
}

var (
	RestrictionsByAPI = make(map[string]Restricted)
	AllowListByAPI    = make(map[string]Allowed)
)

func init() {
	// Initialize restrictions maps
	for _, req := range allRestrictions {
		// Lookup by API method + path combination
		apiKey := string(req.APIMethod) + " " + req.APIPath
		RestrictionsByAPI[apiKey] = req
	}

	// Initialize allow list map
	for _, api := range allowList {
		apiKey := string(api.APIMethod) + " " + api.APIPath
		AllowListByAPI[apiKey] = api
	}
}
