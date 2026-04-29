package system

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/workflow"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/utils/sanitise"
)

var ErrFromAPI = errors.New("failed to transform system from API")

// ToAPIOpt is a functional option for customizing the ToAPI transformation.
type ToAPIOpt func(*cmkapi.System) error

// WithWorkflow sets the workflow field on the API system metadata.
func WithWorkflow(
	ctx context.Context,
	wf *model.Workflow,
	idm identitymanagement.IdentityManagement,
	opts ...workflow.ToAPIOpt,
) ToAPIOpt {
	return func(s *cmkapi.System) error {
		apiWorkflow, err := workflow.ToAPI(ctx, *wf, idm, opts...)
		if s.Metadata == nil {
			s.Metadata = &cmkapi.SystemMetadata{
				Worfklow: apiWorkflow,
			}
		} else {
			s.Metadata.Worfklow = apiWorkflow
		}
		return err
	}
}

// ToAPI transforms a system model to an API system.
func ToAPI(system model.System, systemCfg *config.System, opts ...ToAPIOpt) (*cmkapi.System, error) {
	err := sanitise.Sanitize(&system)
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

	if system.Status == cmkapi.SystemStatusFAILED {
		if system.ErrorCode == "" {
			system.ErrorCode = constants.DefaultErrorCode
		}
		if system.ErrorMessage == "" {
			system.ErrorMessage = constants.DefaultErrorMessage
		}

		apiSystem.Metadata = &cmkapi.SystemMetadata{
			ErrorCode:    &system.ErrorCode,
			ErrorMessage: &system.ErrorMessage,
		}
	}

	// Apply optional transformations
	for _, opt := range opts {
		err := opt(apiSystem)
		if err != nil {
			return nil, err
		}
	}

	return apiSystem, nil
}

func FromAPIPatch(system cmkapi.SystemPatch) model.System {
	return model.System{
		KeyConfigurationID: &system.KeyConfigurationID,
	}
}
