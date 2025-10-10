package mock

import "errors"

var (
	ErrResourceNotFound = errors.New("resource not found")

	ErrCertificateNotFound           = errors.New("certificate not found")
	ErrGroupNotFound                 = errors.New("group not found")
	ErrKeyNotFound                   = errors.New("key not found")
	ErrKeyConfigurationNotFound      = errors.New("key configuration not found")
	ErrKeystoreConfigurationNotFound = errors.New("keystore configuration not found")
	ErrKeyVersionNotFound            = errors.New("key version not found")
	ErrLabelNotFound                 = errors.New("label not found")
	ErrRoleNotFound                  = errors.New("role not found")
	ErrSystemNotFound                = errors.New("system not found")
	ErrTagNotFound                   = errors.New("tag not found")
	ErrTenantNotFound                = errors.New("tenant not found")
	ErrTenantConfigurationNotFound   = errors.New("tenant configuration not found")
	ErrWorkflowNotFound              = errors.New("workflow not found")

	ErrRepoDelete = errors.New("failed to delete")
	ErrRepoFirst  = errors.New("failed to first")
	ErrRepoPatch  = errors.New("failed to patch")

	ErrTransactionFailed = errors.New("transaction failed")
	ErrDbAlreadyExists   = errors.New("database already exists")

	ErrFormatResourceIsNot = errors.New("resource is not ")

	ErrResourceIsNil = errors.New("resource is nil")

	ErrMustPointerToSlice = errors.New("must be a pointer to a slice")
	ErrMustBeSlice        = errors.New("must be a slice")
	ErrItemNotAssignable  = errors.New("item is not assignable")
)
