package operator

import "errors"

var (
	ErrCheckingTenantExistence = errors.New("checking tenant existence failed")
	ErrUninitializedDatabase   = errors.New("database connection not initialized")
	ErrGroupNotFound           = errors.New("group not found")
	ErrSchemaNotFound          = errors.New("schema not found")
	ErrGettingTenantID         = errors.New("error checking tenant ID")
	ErrSendingGroupsFailed     = errors.New("sending groups failed")
)
