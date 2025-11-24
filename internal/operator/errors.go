package operator

import "errors"

var (
	ErrCheckingTenantExistence = errors.New("checking tenant existence failed")
	ErrUninitializedDatabase   = errors.New("database connection not initialized")
	ErrGroupNotFound           = errors.New("group not found")
	ErrSchemaNotFound          = errors.New("schema not found")
	ErrSendingGroupsFailed     = errors.New("sending groups failed")

	ErrMissingProperties = errors.New("missing required properties in auth request")
	ErrMissingIssuer     = errors.New("missing required issuer property")
	ErrMissingTenantID   = errors.New("missing required tenant ID property")
)
