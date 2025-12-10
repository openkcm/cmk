package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/openkcm/orbital"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/event-processor/proto"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
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

type Event struct {
	Name  string
	Event func(ctx context.Context) (orbital.Job, error)
}

var ErrEventSendingFailed = errors.New("failed to send event")

func (c *CryptoReconciler) SendEvent(ctx context.Context, event Event) error {
	if c == nil {
		return errs.Wrapf(ErrEventSendingFailed, "reconciler is not initialized")
	}

	ctx = log.InjectSystemEvent(ctx, event.Name)

	job, err := event.Event(ctx)
	if err != nil {
		return errs.Wrap(ErrEventSendingFailed, err)
	}

	log.Info(ctx, "Event Sent", slog.String("JobID", job.ID.String()))

	return nil
}

var (
	ErrNoPreviousEvent  = errors.New("no previous events found for selected item")
	ErrSystemProcessing = errors.New("system is still in processing state")

	ErrMissingKeyID = errors.New("keyID is required to create key event job")
)

// GetLastEvent returns the last event of an item
func (c *CryptoReconciler) GetLastEvent(
	ctx context.Context,
	cmkItemID string,
) (*model.Event, error) {
	job := &model.Event{}

	query := *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IdentifierField, cmkItemID)))

	found, err := c.repo.First(ctx, job, query)
	if err != nil || !found {
		return nil, errs.Wrap(ErrNoPreviousEvent, err)
	}

	return job, nil
}

func (c *CryptoReconciler) handleSystemStatus(
	ctx context.Context,
	system *model.System,
	eventFn func() (orbital.Job, error),
) (orbital.Job, error) {
	job := orbital.Job{}
	previousStatus := system.Status

	err := c.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		if system.Status == cmkapi.SystemStatusPROCESSING {
			return ErrSystemProcessing
		}

		system.Status = cmkapi.SystemStatusPROCESSING

		_, err := r.Patch(ctx, system, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return err
		}

		job, err = eventFn()

		return err
	})
	if err != nil {
		return job, err
	}

	// Dont store event information if non user triggered
	if cmkcontext.IsSystemUser(ctx) {
		return job, nil
	}

	err = c.repo.Set(ctx, &model.Event{
		Identifier:         job.ExternalID,
		Type:               job.Type,
		Data:               job.Data,
		Status:             job.Status,
		PreviousItemStatus: string(previousStatus),
	})
	if err != nil {
		log.Error(ctx, "failed to store event", err)
	}

	return job, nil
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

// KeyDetach creates a job to detach a key.
// Context provided must have the tenant set.
func (c *CryptoReconciler) KeyDetach(ctx context.Context, keyID string) (orbital.Job, error) {
	return c.createKeyEventJob(ctx, keyID, proto.TaskType_KEY_DETACH)
}

func (c *CryptoReconciler) createKeyEventJob(
	ctx context.Context,
	keyID string,
	taskType proto.TaskType,
) (orbital.Job, error) {
	if keyID == "" {
		return orbital.Job{}, ErrMissingKeyID
	}

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
		Identifier: data.SystemID,
		Type:       taskType.String(),
		Data:       jobData,
	}

	return c.CreateJob(ctx, event)
}
