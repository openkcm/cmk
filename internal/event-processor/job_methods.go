package eventprocessor

import (
	"context"
	"encoding/json"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/event-processor/proto"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// KeyActionJobData contains the data needed for a key action orbital job.
type KeyActionJobData struct {
	TenantID string `json:"tenantID"`
	KeyID    string `json:"keyID"`
}

// SystemActionJobData contains the data needed for a system action orbital job.
type SystemActionJobData struct {
	SystemID string `json:"systemID"`
	TenantID string `json:"tenantID"`
	KeyID    string `json:"keyID"`
}

// SystemLink creates a job to link a system with a key make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) SystemLink(ctx context.Context, systemID string, keyID string) (orbital.Job, error) {
	return c.createSystemEventJob(ctx, systemID, keyID, proto.TaskType_SYSTEM_LINK)
}

func (c *CryptoReconciler) SystemUnlink(ctx context.Context, systemID string, keyID string) (orbital.Job, error) {
	return c.createSystemEventJob(ctx, systemID, keyID, proto.TaskType_SYSTEM_UNLINK)
}

// KeyEnable creates a job to enable a key make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) KeyEnable(ctx context.Context, keyID string) (orbital.Job, error) {
	return c.createKeyEventJob(ctx, keyID, proto.TaskType_KEY_ENABLE)
}

// KeyDisable creates a job to disable a key make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) KeyDisable(ctx context.Context, keyID string) (orbital.Job, error) {
	return c.createKeyEventJob(ctx, keyID, proto.TaskType_KEY_DISABLE)
}

func (c *CryptoReconciler) createKeyEventJob(
	ctx context.Context,
	keyID string,
	taskType proto.TaskType,
) (orbital.Job, error) {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return orbital.Job{}, err
	}

	data := KeyActionJobData{
		TenantID: tenantID,
		KeyID:    keyID,
	}

	jobData, err := json.Marshal(data)
	if err != nil {
		return orbital.Job{}, err
	}

	return c.createJob(ctx, jobData, taskType.String())
}

func (c *CryptoReconciler) createSystemEventJob(
	ctx context.Context,
	systemID string,
	keyID string,
	taskType proto.TaskType,
) (orbital.Job, error) {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return orbital.Job{}, err
	}

	data := SystemActionJobData{
		SystemID: systemID,
		TenantID: tenantID,
		KeyID:    keyID,
	}

	jobData, err := json.Marshal(data)
	if err != nil {
		return orbital.Job{}, err
	}

	return c.createJob(ctx, jobData, taskType.String())
}
