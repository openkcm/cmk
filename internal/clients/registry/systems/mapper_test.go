package systems_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	v1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/types/v1"

	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/model"
)

func Test_MapRegistrySystemsToCmkSystems(t *testing.T) {
	type args struct {
		grpcSystems []*systemgrpc.System
	}

	tests := []struct {
		name string
		args args
		want []*model.System
	}{
		{
			name: "should map grpc system to model system",
			args: args{
				grpcSystems: []*systemgrpc.System{
					{
						ExternalId:    "id",
						TenantId:      "tenantId",
						L2KeyId:       "l2keyId",
						HasL1KeyClaim: false,
						Region:        string(v1.Region_REGION_EU),
						Type:          string(systems.SystemTypeSYSTEM),
						UpdatedAt:     "time",
						CreatedAt:     "time",
					},
				},
			},
			want: []*model.System{
				{
					Identifier: "id",
					Region:     string(v1.Region_REGION_EU),
					Type:       string(systems.SystemTypeSYSTEM),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedSystems, err := systems.MapRegistrySystemsToCmkSystems(tt.args.grpcSystems)
			assert.NoError(t, err)

			tt.want[0].ID = mappedSystems[0].ID
			assert.Equalf(t, tt.want, mappedSystems, "mapRegistrySystemToCmkSystem(%v)", tt.args.grpcSystems)
		})
	}
}
