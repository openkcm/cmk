package eventprocessor

import (
	"context"
	"fmt"
	"strings"

	"github.com/openkcm/orbital"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	protoPkg "google.golang.org/protobuf/proto"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// SystemTaskInfoResolver is responsible for resolving the necessary information to create a TaskInfo
// for system-related tasks such as linking and unlinking systems.
type SystemTaskInfoResolver struct {
	repo        repo.Repo
	targets     map[string]struct{}
	svcRegistry *cmkpluginregistry.Registry
}

func (r *SystemTaskInfoResolver) GetTaskInfo(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
) (*orbital.TaskInfo, error) {
	tenant, err := getTenantByID(ctx, r.repo, data.TenantID)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	system, err := getSystemByID(ctx, r.repo, data.SystemID)
	if err != nil {
		return nil, err
	}

	_, ok := r.targets[system.Region]
	if !ok {
		return nil, errs.Wrapf(ErrTargetNotConfigured, system.Region)
	}

	keyID := data.KeyIDTo
	if taskType == proto.TaskType_SYSTEM_UNLINK {
		keyID = data.KeyIDFrom
	}

	key, err := getKeyByKeyID(ctx, r.repo, keyID)
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, err := r.getKeyAccessMetadata(ctx, *key, system.Region)
	if err != nil {
		return nil, err
	}

	taskData := &proto.Data{
		TaskType: taskType,
		Data: &proto.Data_SystemAction{
			SystemAction: &proto.SystemAction{
				SystemId:          system.Identifier,
				SystemRegion:      system.Region,
				SystemType:        strings.ToLower(system.Type),
				KeyIdFrom:         data.KeyIDFrom,
				KeyIdTo:           data.KeyIDTo,
				KeyProvider:       strings.ToLower(key.Provider),
				TenantId:          tenant.ID,
				TenantOwnerId:     tenant.OwnerID,
				TenantOwnerType:   tenant.OwnerType,
				CmkRegion:         tenant.Region,
				KeyAccessMetaData: keyAccessMetadata,
			},
		},
	}

	taskDataBytes, err := protoPkg.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %w", err)
	}

	return &orbital.TaskInfo{
		Target: system.Region,
		Data:   taskDataBytes,
		Type:   taskType.String(),
	}, nil
}

func (r *SystemTaskInfoResolver) getKeyAccessMetadata(
	ctx context.Context,
	key model.Key,
	systemRegion string,
) ([]byte, error) {
	plugin := r.svcRegistry.LookupByTypeAndName(keystoreopv1.Type, key.Provider)
	if plugin == nil {
		return nil, ErrPluginNotFound
	}

	cryptoAccessData, err := keystoreopv1.NewKeystoreInstanceKeyOperationClient(plugin.ClientConnection()).
		TransformCryptoAccessData(
			ctx,
			&keystoreopv1.TransformCryptoAccessDataRequest{
				NativeKeyId: *key.NativeID,
				AccessData:  key.CryptoAccessData,
			})
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, ok := cryptoAccessData.GetTransformedAccessData()[systemRegion]
	if !ok {
		return nil, ErrKeyAccessMetadataNotFound
	}

	return keyAccessMetadata, nil
}

// KeyTaskInfoResolver is responsible for resolving the necessary information to create a TaskInfo
// for key-related tasks such as enabling, disabling, detaching.
type KeyTaskInfoResolver struct {
	repo    repo.Repo
	targets map[string]struct{}
}

func (r *KeyTaskInfoResolver) GetTaskInfo(
	ctx context.Context,
	taskType proto.TaskType,
	data KeyActionJobData,
) ([]orbital.TaskInfo, error) {
	tenant, err := getTenantByID(ctx, r.repo, data.TenantID)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	var targets map[string]struct{}
	switch taskType {
	case proto.TaskType_KEY_ENABLE, proto.TaskType_KEY_DISABLE:
		regions, err := r.getRegionsByKeyID(ctx, data.KeyID)
		if err != nil {
			return nil, err
		}
		if len(regions) == 0 {
			return nil, ErrNoConnectedRegionsForKey
		}
		targets = regions
	default:
		targets = r.targets
	}

	result := make([]orbital.TaskInfo, 0, len(targets))

	for target := range targets {
		taskData := &proto.Data{
			TaskType: taskType,
			Data: &proto.Data_KeyAction{
				KeyAction: &proto.KeyAction{
					KeyId:     data.KeyID,
					TenantId:  tenant.ID,
					CmkRegion: tenant.Region,
				},
			},
		}

		taskDataBytes, err := protoPkg.Marshal(taskData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal task data: %w", err)
		}

		result = append(result, orbital.TaskInfo{
			Target: target,
			Data:   taskDataBytes,
			Type:   taskType.String(),
		})
	}

	return result, nil
}

// getRegionsByKeyID gets all distinct regions with CONNECTED systems for a given key ID.
func (r *KeyTaskInfoResolver) getRegionsByKeyID(ctx context.Context, keyID string) (map[string]struct{}, error) {
	key := &model.Key{}
	_, err := r.repo.First(ctx, key, *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IDField, keyID),
		),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to get key by ID %s: %w", keyID, err)
	}

	regions := make(map[string]struct{})

	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.KeyConfigIDField, key.KeyConfigurationID),
		),
	)
	err = repo.ProcessInBatch(ctx, r.repo, query, repo.DefaultLimit, func(systems []*model.System) error {
		for _, system := range systems {
			if system.Status == cmkapi.SystemStatusCONNECTED {
				if _, ok := r.targets[system.Region]; !ok {
					ctx := model.LogInjectSystem(ctx, system)
					log.Error(ctx,
						"skipping region for connected system as target is not configured", ErrUnsupportedRegion)
					continue
				}
				regions[system.Region] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get connected regions for key ID %s: %w", keyID, err)
	}

	return regions, nil
}
