package system

import (
	"errors"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

var ErrFromAPI = errors.New("failed to transform system from API")

// ToAPI transforms a system model to an API system.
func ToAPI(system model.System, systemCfg *config.System) (*cmkapi.System, error) {
	err := sanitise.Stringlikes(&system)
	if err != nil {
		return nil, err
	}

	var properties map[string]any
	if len(system.Properties) > 0 {
		properties = make(map[string]any, len(system.Properties))
		for k, v := range system.Properties {
			_, ok := systemCfg.OptionalProperties[k]
			// Only show in UI fields that exist on the configuration
			if !ok {
				continue
			}

			properties[k] = v
		}
	}

	apiSystem := &cmkapi.System{
		ID:                   &system.ID,
		Identifier:           &system.Identifier,
		Region:               system.Region,
		Properties:           &properties,
		Type:                 system.Type,
		KeyConfigurationID:   system.KeyConfigurationID,
		KeyConfigurationName: system.KeyConfigurationName,
		Status:               system.Status,
	}

	return apiSystem, nil
}

func FromAPIPatch(system cmkapi.SystemPatch) model.System {
	return model.System{
		KeyConfigurationID: &system.KeyConfigurationID,
	}
}
