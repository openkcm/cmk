package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openkcm/orbital"

	protoPkg "google.golang.org/protobuf/proto"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// SystemTaskInfoResolver is responsible for resolving the necessary information to create a TaskInfo
// for system-related tasks such as linking and unlinking systems.
type SystemTaskInfoResolver struct {
	repo        repo.Repo
	targets     map[string]struct{}
	svcRegistry serviceapi.Registry
	cfg         *config.Config
}

func (r *SystemTaskInfoResolver) Resolve(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	data, err := unmarshalSystemJobData(orbital.Job{Data: job.Data})
	if err != nil {
		return nil, err
	}

	var taskType proto.TaskType
	switch JobType(job.Type) {
	case JobTypeSystemLink:
		taskType = proto.TaskType_SYSTEM_LINK
	case JobTypeSystemUnlink, JobTypeSystemUnlinkDecommission:
		taskType = proto.TaskType_SYSTEM_UNLINK
	case JobTypeSystemSwitch, JobTypeSystemSwitchNewPK:
		taskType = proto.TaskType_SYSTEM_SWITCH
	case JobTypeSystemKeyRotate:
		taskType = proto.TaskType_SYSTEM_KEY_ROTATE
	default:
		return nil, errs.Wrapf(ErrInvalidJobType, job.Type)
	}

	taskInfo, err := r.getTaskInfo(ctx, taskType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task info: %w", err)
	}

	return []orbital.TaskInfo{*taskInfo}, nil
}

func (r *SystemTaskInfoResolver) getTaskInfo(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
) (*orbital.TaskInfo, error) {
	tenant, system, err := r.loadTenantAndSystem(ctx, data)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	if err := r.validateSystemRegionTarget(system.Region); err != nil {
		return nil, err
	}

	key, keyAccessMetadata, err := r.getKeyAndAccessMetadata(ctx, taskType, data, system.Region)
	if err != nil {
		return nil, err
	}

	taskData := r.buildSystemActionTaskData(taskType, data, tenant, system, key, keyAccessMetadata)

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

func (r *SystemTaskInfoResolver) validateSystemRegionTarget(region string) error {
	if _, ok := r.targets[region]; !ok {
		return errs.Wrapf(ErrTargetNotConfigured, region)
	}
	return nil
}

func (r *SystemTaskInfoResolver) getKeyAndAccessMetadata(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
	systemRegion string,
) (*model.Key, []byte, error) {
	key, err := r.selectKeyForTask(ctx, taskType, data)
	if err != nil {
		return nil, nil, err
	}

	keyAccessMetadata, err := r.getKeyAccessMetadata(ctx, *key, systemRegion)
	if err != nil {
		return nil, nil, err
	}

	return key, keyAccessMetadata, nil
}

func (r *SystemTaskInfoResolver) buildSystemActionTaskData(
	taskType proto.TaskType,
	data SystemActionJobData,
	tenant *model.Tenant,
	system *model.System,
	key *model.Key,
	keyAccessMetadata []byte,
) *proto.Data {
	return &proto.Data{
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
				CmkRegion:         r.cfg.Landscape.Region,
				KeyAccessMetaData: keyAccessMetadata,
			},
		},
	}
}

func (r *SystemTaskInfoResolver) loadTenantAndSystem(
	ctx context.Context,
	data SystemActionJobData,
) (*model.Tenant, *model.System, error) {
	tenant, err := getTenantByID(ctx, r.repo, data.TenantID)
	if err != nil {
		return nil, nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	system, err := getSystemByID(ctx, r.repo, data.SystemID)
	if err != nil {
		return nil, nil, err
	}

	return tenant, system, nil
}

func (r *SystemTaskInfoResolver) selectKeyForTask(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
) (*model.Key, error) {
	keyID := data.KeyIDTo
	if taskType == proto.TaskType_SYSTEM_UNLINK {
		keyID = data.KeyIDFrom
	}

	if taskType == proto.TaskType_SYSTEM_KEY_ROTATE && data.KeyIDFrom != data.KeyIDTo {
		return nil, fmt.Errorf("%w: got %q -> %q", ErrKeyRotateMismatchedKeyIDs, data.KeyIDFrom, data.KeyIDTo)
	}

	key, err := getKeyByKeyID(ctx, r.repo, keyID)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// fetchAndPopulateVersionInfo fetches the newest key version from the DB
// and populates its native ID into the crypto access data for all regions.
// If no key version exists, the crypto access data is returned unchanged for backward compatibility.
func (r *SystemTaskInfoResolver) fetchAndPopulateVersionInfo(
	ctx context.Context,
	key model.Key,
) (map[string]map[string]any, error) {
	cryptoData := key.GetCryptoAccessData()

	latestVersionID, err := getNewestKeyVersionNativeID(ctx, r.repo, key.ID.String())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return cryptoData, nil
		}
		return nil, fmt.Errorf("failed to get latest key version native ID: %w", err)
	}

	if latestVersionID == "" {
		return cryptoData, nil
	}

	for region := range cryptoData {
		if cryptoData[region] == nil {
			// Initialize map if it doesn't exist
			cryptoData[region] = make(map[string]any)
		}
		cryptoData[region]["versionIdentifier"] = latestVersionID
	}

	return cryptoData, nil
}

func (r *SystemTaskInfoResolver) getKeyAccessMetadata(
	ctx context.Context,
	key model.Key,
	systemRegion string,
) ([]byte, error) {
	keyManagements, err := r.svcRegistry.KeyManagements()
	if err != nil {
		return nil, ErrPluginNotFound
	}

	client, ok := keyManagements[key.Provider]
	if !ok {
		return nil, ErrPluginNotFound
	}

	// Fetch and populate version info
	cryptoData, err := r.fetchAndPopulateVersionInfo(ctx, key)
	if err != nil {
		return nil, err
	}

	// Marshal updated crypto data
	cryptoAccessDataBytes, err := json.Marshal(cryptoData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal crypto access data: %w", err)
	}

	// Transform with fresh version data
	transformedData, err := client.TransformCryptoAccessData(
		ctx,
		&keymanagement.TransformCryptoAccessDataRequest{
			NativeKeyID: *key.NativeID,
			AccessData:  cryptoAccessDataBytes,
		})
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, ok := transformedData.TransformedAccessData[systemRegion]
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
	cfg     *config.Config
}

func (r *KeyTaskInfoResolver) Resolve(
	ctx context.Context,
	job orbital.Job,
) ([]orbital.TaskInfo, error) {
	data, err := unmarshalKeyJobData(orbital.Job{Data: job.Data})
	if err != nil {
		return nil, err
	}

	var taskType proto.TaskType
	switch JobType(job.Type) {
	case JobTypeKeyEnable:
		taskType = proto.TaskType_KEY_ENABLE
	case JobTypeKeyDisable:
		taskType = proto.TaskType_KEY_DISABLE
	case JobTypeKeyDetach:
		taskType = proto.TaskType_KEY_DETACH
	case JobTypeKeyDelete:
		taskType = proto.TaskType_KEY_DELETE
	default:
		return nil, errs.Wrapf(ErrInvalidJobType, job.Type)
	}

	taskInfos, err := r.getTaskInfo(ctx, taskType, data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task info: %w", err)
	}

	return taskInfos, nil
}

func (r *KeyTaskInfoResolver) getTaskInfo(
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
					CmkRegion: r.cfg.Landscape.Region,
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
