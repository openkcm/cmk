package eventprocessor

import (
	"context"

	"github.com/openkcm/orbital"
)

type JobType string

func (t JobType) String() string {
	return string(t)
}

const (
	JobTypeSystemLink        JobType = "SYSTEM_LINK"
	JobTypeSystemUnlink      JobType = "SYSTEM_UNLINK"
	JobTypeSystemSwitch      JobType = "SYSTEM_SWITCH"
	JobTypeSystemSwitchNewPK JobType = "SYSTEM_SWITCH_NEW_PK"
	JobTypeKeyEnable         JobType = "KEY_ENABLE"
	JobTypeKeyDisable        JobType = "KEY_DISABLE"
	JobTypeKeyDetach         JobType = "KEY_DETACH"
	JobTypeKeyDelete         JobType = "KEY_DELETE"
	JobTypeKeyRotate         JobType = "KEY_ROTATE"
)

type JobHandler interface {
	ResolveTasks(ctx context.Context, job orbital.Job) ([]orbital.TaskInfo, error)
	HandleJobConfirm(ctx context.Context, job orbital.Job) (orbital.JobConfirmerResult, error)
	HandleJobDoneEvent(ctx context.Context, job orbital.Job) error
	HandleJobFailedEvent(ctx context.Context, job orbital.Job) error
	HandleJobCanceledEvent(ctx context.Context, job orbital.Job) error
}

// KeyActionJobData contains the data needed for a key action orbital job.
type KeyActionJobData struct {
	TenantID string `json:"tenantID"`
	KeyID    string `json:"keyID"`
}

// SystemActionJobData contains the data needed for a system action orbital job.
type SystemActionJobData struct {
	SystemID  string `json:"systemID"`
	TenantID  string `json:"tenantID"`
	KeyIDTo   string `json:"keyIDTo"`
	KeyIDFrom string `json:"keyIDFrom"`
	Trigger   string `json:"trigger,omitempty"`
}

// KeyConfigActionJobData contains the data needed for a key configuration action orbital job.
type KeyConfigActionJobData struct {
	TenantID    string `json:"tenantID"`
	KeyConfigID string `json:"keyConfigID"`
}
