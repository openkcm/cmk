package systems

import (
	"github.com/google/uuid"

	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.com/openkcm/cmk/internal/model"
)

func MapRegistrySystemsToCmkSystems(grpcSystems []*systemgrpc.System) ([]*model.System, error) {
	systems := make([]*model.System, len(grpcSystems))

	for i, grpcSystem := range grpcSystems {
		system, err := mapRegistrySystemToCmkSystem(grpcSystem)
		if err != nil {
			return nil, err
		}

		systems[i] = system
	}

	return systems, nil
}

func mapRegistrySystemToCmkSystem(grpcSystem *systemgrpc.System) (*model.System, error) {
	systemType, err := systemTypeStringMap(grpcSystem.GetType())
	if err != nil {
		return nil, err
	}

	return &model.System{
		ID:         uuid.New(),
		Region:     grpcSystem.GetRegion(),
		Identifier: grpcSystem.GetExternalId(),
		Type:       string(*systemType),
	}, nil
}
