package model

import (
	"errors"

	pb "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
)

var (
	ErrInvalidTenantStatus = errors.New("tenant status is not valid")

	// validStatuses contains all valid tenant statuses. Calculated in the init().
	validTenantStatuses = map[TenantStatus]struct{}{}
)

// TenantStatus represents the status of the tenant.
type TenantStatus string

// init calculates valid tenant status.
func init() {
	for _, v := range pb.Status_name {
		validTenantStatuses[TenantStatus(v)] = struct{}{}
	}
}

// Validate validates the given status of the tenant.
// Returns an error if the status is invalid.
func (s TenantStatus) Validate() error {
	if _, ok := validTenantStatuses[s]; !ok {
		return ErrInvalidTenantStatus
	}

	return nil
}
