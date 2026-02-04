package authz

type Operation string

const (
	OpCreate Operation = "CREATE"
	OpList   Operation = "LIST"
	OpDelete Operation = "DELETE"
	OpFirst  Operation = "FIRST"
	OpPatch  Operation = "PATCH"
	OpSet    Operation = "SET"
)

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
	APIPath          string
	APIMethod        APIMethod
	ResourceTypeName ResourceTypeName
	Action           Action
	RepoOperation    Operation
}

// Define all restrictions once
var allRestrictions = []Restricted{
	// Keys endpoints
	{
		APIPath:          "/keys",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/keys",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/keys/{keyID}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/keys/{keyID}",
		APIMethod:        APIMethodPatch,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionUpdate,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/keys/{keyID}",
		APIMethod:        APIMethodDelete,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionDelete,
		RepoOperation:    OpDelete,
	},
	{
		APIPath:          "/keys/{keyID}/importParams",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/keys/{keyID}/importKeyMaterial",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionUpdate,
		RepoOperation:    OpSet,
	},
	{
		APIPath:          "/keys/{keyID}/versions",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/keys/{keyID}/versions",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/keys/{keyID}/versions/{version}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/key/{keyID}/labels",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/key/{keyID}/labels",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/key/{keyID}/label/{labelName}",
		APIMethod:        APIMethodDelete,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionDelete,
		RepoOperation:    OpDelete,
	},

	// Key Configurations endpoints
	{
		APIPath:          "/keyConfigurations",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/keyConfigurations",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}",
		APIMethod:        APIMethodPatch,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionUpdate,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}",
		APIMethod:        APIMethodDelete,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionDelete,
		RepoOperation:    OpDelete,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}/tags",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}/tags",
		APIMethod:        APIMethodPut,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionUpdate,
		RepoOperation:    OpSet,
	},
	{
		APIPath:          "/keyConfigurations/{keyConfigurationID}/certificates",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKeyConfiguration,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},

	// Systems endpoints
	{
		APIPath:          "/systems",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/systems/{systemID}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/systems/{systemID}/link",
		APIMethod:        APIMethodPatch,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionSystemModifyLink,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/systems/{systemID}/link",
		APIMethod:        APIMethodDelete,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionSystemModifyLink,
		RepoOperation:    OpDelete,
	},
	{
		APIPath:          "/systems/{systemID}/recoveryActions",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionSystemModifyLink,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/systems/{systemID}/recoveryActions",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeSystem,
		Action:           ActionSystemModifyLink,
		RepoOperation:    OpList,
	},

	// Workflows endpoints
	{
		APIPath:          "/workflows",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/workflows",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/workflows/check",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionUpdate,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/workflows/{workflowID}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/workflows/{workflowID}/approvers",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/workflows/{workflowID}/approvers",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/workflows/{workflowID}/state",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeWorkFlow,
		Action:           ActionUpdate,
		RepoOperation:    OpPatch,
	},

	// Groups endpoints
	{
		APIPath:          "/groups",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionRead,
		RepoOperation:    OpList,
	},
	{
		APIPath:          "/groups",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionCreate,
		RepoOperation:    OpCreate,
	},
	{
		APIPath:          "/groups/iamCheck",
		APIMethod:        APIMethodPost,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/groups/{groupID}",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/groups/{groupID}",
		APIMethod:        APIMethodPatch,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionUpdate,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/groups/{groupID}",
		APIMethod:        APIMethodDelete,
		ResourceTypeName: ResourceTypeUserGroup,
		Action:           ActionDelete,
		RepoOperation:    OpDelete,
	},

	// Tenant endpoints
	{
		APIPath:          "/tenantConfigurations/keystores",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeKey,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/tenantConfigurations/workflow",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeTenantSettings,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
	{
		APIPath:          "/tenantConfigurations/workflow",
		APIMethod:        APIMethodPatch,
		ResourceTypeName: ResourceTypeTenantSettings,
		Action:           ActionUpdate,
		RepoOperation:    OpPatch,
	},
	{
		APIPath:          "/tenantInfo",
		APIMethod:        APIMethodGet,
		ResourceTypeName: ResourceTypeTenant,
		Action:           ActionRead,
		RepoOperation:    OpFirst,
	},
}

var (
	RestrictionsByOperation = make(map[Operation]Restricted)
	RestrictionsByAPI       = make(map[string]Restricted)
	AllowListByAPI          = make(map[string]Allowed)
)

func init() {
	// Initialize restrictions maps
	for _, req := range allRestrictions {
		// Lookup by action name
		RestrictionsByOperation[req.RepoOperation] = req

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
