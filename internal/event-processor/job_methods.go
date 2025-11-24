package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

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
}

var (
	ErrNoPreviousEvent  = errors.New("no previous events found for selected item")
	ErrSystemProcessing = errors.New("system is still in processing state")
)

// GetLastEvent returns the last event of an item
func (c *CryptoReconciler) GetLastEvent(
	ctx context.Context,
	eventTypes []string,
	cmkItemID string,
) (*model.Event, error) {
	var jobs []*model.Event

	identifierCond := repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IdentifierField, cmkItemID))
	typesCond := repo.NewCompositeKey()

	typesCond.IsStrict = false
	for _, typ := range eventTypes {
		typesCond = typesCond.Where(repo.TypeField, typ)
	}

	count, err := c.repo.List(
		ctx,
		model.Event{},
		&jobs,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(typesCond)).Where(identifierCond).
			Order(repo.OrderField{
				Field:     repo.CreatedField,
				Direction: repo.Desc,
			}),
	)
	if err != nil || count < 1 {
		return nil, errs.Wrap(ErrNoPreviousEvent, err)
	}

	return jobs[0], nil
}

func (c *CryptoReconciler) handleSystemStatus(
	ctx context.Context,
	s *model.System,
	f func() (orbital.Job, error),
) (orbital.Job, error) {
	job := orbital.Job{}
	err := c.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		if s.Status == cmkapi.SystemStatusPROCESSING {
			return ErrSystemProcessing
		}

		s.Status = cmkapi.SystemStatusPROCESSING

		_, err := r.Patch(ctx, s, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return err
		}

		job, err = f()

		return err
	})

	return job, err
}

// SystemLink creates a job to link a system with a key make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) SystemLink(ctx context.Context, system *model.System, keyID string) (orbital.Job, error) {
	return c.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemLinkJobData := SystemActionJobData{
			SystemID: system.ID.String(),
			KeyIDTo:  keyID,
		}

		return c.createSystemEventJob(ctx, proto.TaskType_SYSTEM_LINK, systemLinkJobData)
	})
}

// SystemUnlink creates a job to unlink a system from a key make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) SystemUnlink(ctx context.Context, system *model.System, keyID string) (orbital.Job, error) {
	return c.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemUnlinkJobData := SystemActionJobData{
			SystemID:  system.ID.String(),
			KeyIDFrom: keyID,
		}

		return c.createSystemEventJob(ctx, proto.TaskType_SYSTEM_UNLINK, systemUnlinkJobData)
	})
}

// SystemSwitch creates a job to switch the key of a system from keyIDFrom to keyIDTo
// make sure the ctx provided has the tenant set.
func (c *CryptoReconciler) SystemSwitch(
	ctx context.Context,
	system *model.System,
	keyIDTo string,
	keyIDFrom string,
) (orbital.Job, error) {
	return c.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemSwitchJobData := SystemActionJobData{
			SystemID:  system.ID.String(),
			KeyIDTo:   keyIDTo,
			KeyIDFrom: keyIDFrom,
		}

		return c.createSystemEventJob(ctx, proto.TaskType_SYSTEM_SWITCH, systemSwitchJobData)
	})
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

	event := &model.Event{
		Identifier: keyID,
		Type:       taskType.String(),
		Data:       jobData,
	}

	return c.CreateJob(ctx, event)
}

func (c *CryptoReconciler) createSystemEventJob(
	ctx context.Context,
	taskType proto.TaskType,
	data SystemActionJobData,
) (orbital.Job, error) {
	systemID := data.SystemID

	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return orbital.Job{}, err
	}

	data.TenantID = tenantID

	jobData, err := json.Marshal(data)
	if err != nil {
		return orbital.Job{}, err
	}

	event := &model.Event{
		Identifier: systemID,
		Type:       taskType.String(),
		Data:       jobData,
	}

	return c.CreateJob(ctx, event)
}
