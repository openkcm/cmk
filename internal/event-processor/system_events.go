package eventprocessor

import (
	"context"
	"fmt"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"
	"strings"

	"github.com/openkcm/orbital"
	protoPkg "google.golang.org/protobuf/proto"

	"github.com/openkcm/cmk/internal/event-processor/proto"
)

type SystemJobHandler struct {
	repo        repo.Repo
	targets     map[string]struct{}
	svcRegistry *cmkpluginregistry.Registry
	cmkAuditor  *auditor.Auditor
}

func (h *SystemJobHandler) getSystemActionData(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
) (*orbital.TaskInfo, error) {
	tenant, err := getTenantByID(ctx, h.repo, data.TenantID)
	if err != nil {
		return nil, err
	}

	ctx = cmkcontext.CreateTenantContext(ctx, data.TenantID)

	system, err := getSystemByID(ctx, h.repo, data.SystemID)
	if err != nil {
		return nil, err
	}

	_, ok := h.targets[system.Region]
	if !ok {
		return nil, errs.Wrapf(ErrTargetNotConfigured, system.Region)
	}

	keyID := data.KeyIDTo
	if taskType == proto.TaskType_SYSTEM_UNLINK {
		keyID = data.KeyIDFrom
	}

	key, err := getKeyByKeyID(ctx, h.repo, keyID)
	if err != nil {
		return nil, err
	}

	keyAccessMetadata, err := h.getKeyAccessMetadata(ctx, *key, system.Region)
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

func (h *SystemJobHandler) getKeyAccessMetadata(
	ctx context.Context,
	key model.Key,
	systemRegion string,
) ([]byte, error) {
	plugin := h.svcRegistry.LookupByTypeAndName(keystoreopv1.Type, key.Provider)
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
