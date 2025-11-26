package model

import (
	"errors"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

var (
	ErrInvalidTenantRole = errors.New("tenant role is not valid")

	// validTenantRoles contains all valid tenant roles. Calculated in the init().
	validTenantRoles = map[TenantRole]struct{}{}
)

// TenantRole represents the role of the tenant.
type TenantRole string

// init calculates valid tenant roles.
func init() {
	for _, v := range pb.Role_name {
		validTenantRoles[TenantRole(v)] = struct{}{}
	}
}

// Validate validates the given role of the tenant.
// Returns an error if the status is invalid.
func (s TenantRole) Validate() error {
	if _, ok := validTenantRoles[s]; !ok {
		return ErrInvalidTenantRole
	}

	return nil
}
