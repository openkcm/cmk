package systems_test

import (
	"strings"

	"github.com/google/uuid"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	regionpb "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"
)

func randExternalID() string {
	id1 := strings.ReplaceAll(uuid.New().String(), "-", "")
	id2 := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]

	return id1 + id2
}

func validRegisterSystemReq(b bool) *systemgrpc.RegisterSystemRequest {
	return &systemgrpc.RegisterSystemRequest{
		ExternalId:    randExternalID(),
		L2KeyId:       "key123",
		Region:        string(regionpb.Region_REGION_EU),
		Type:          "SYSTEM",
		HasL1KeyClaim: b,
	}
}
