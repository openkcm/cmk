package eventprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrEventSendingFailed = errors.New("failed to send event")

	ErrNoPreviousEvent  = errors.New("no previous events found for selected item")
	ErrSystemProcessing = errors.New("system is still in processing state")

	ErrMissingKeyID = errors.New("keyID is required to create key event job")
)

type Event struct {
	Name  string
	Event func(ctx context.Context) (orbital.Job, error)
}

type EventFactory struct {
	repo    repo.Repo
	manager *orbital.Manager
}

func NewEventFactory(
	ctx context.Context,
	cfg *config.Config,
	repository repo.Repo,
) (*EventFactory, error) {
	orbRepo, err := createOrbitalRepository(ctx, cfg.Database)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital repository")
	}

	// Orbital manager is used to create jobs that will be processed by the orbital worker.
	// It is not used for processing jobs in the event processor, so we can pass nil for the handler.
	manager, err := orbital.NewManager(orbRepo, dummyResolveTask)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create orbital manager")
	}

	return &EventFactory{
		repo:    repository,
		manager: manager,
	}, nil
}

func (f *EventFactory) CreateJob(ctx context.Context, event *model.Event) (orbital.Job, error) {
	job := orbital.NewJob(event.Type, event.Data).WithExternalID(event.Identifier)
	return f.manager.PrepareJob(ctx, job)
}

func (f *EventFactory) SendEvent(ctx context.Context, event Event) error {
	ctx = log.InjectSystemEvent(ctx, event.Name)

	job, err := event.Event(ctx)
	if err != nil {
		return errs.Wrap(ErrEventSendingFailed, err)
	}

	log.Info(ctx, "Event Sent", slog.String("jobId", job.ID.String()))

	return nil
}

// GetLastEvent returns the last event of an item
func (f *EventFactory) GetLastEvent(
	ctx context.Context,
	cmkItemID string,
) (*model.Event, error) {
	job := &model.Event{}

	query := *repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IdentifierField, cmkItemID)))

	found, err := f.repo.First(ctx, job, query)
	if err != nil || !found {
		return nil, errs.Wrap(ErrNoPreviousEvent, err)
	}

	return job, nil
}

// SystemLink creates a job to link a system with a key make sure the ctx provided has the tenant set.
func (f *EventFactory) SystemLink(ctx context.Context, system *model.System, keyID string) (orbital.Job, error) {
	return f.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemLinkJobData := SystemActionJobData{
			SystemID: system.ID.String(),
			KeyIDTo:  keyID,
		}

		return f.createSystemEventJob(ctx, JobTypeSystemLink, systemLinkJobData)
	})
}

// SystemUnlink creates a job to unlink a system from a key make sure the ctx provided has the tenant set.
func (f *EventFactory) SystemUnlink(
	ctx context.Context,
	system *model.System,
	keyID string,
	trigger string,
) (orbital.Job, error) {
	return f.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemUnlinkJobData := SystemActionJobData{
			SystemID:  system.ID.String(),
			KeyIDFrom: keyID,
			Trigger:   trigger,
		}

		return f.createSystemEventJob(ctx, JobTypeSystemUnlink, systemUnlinkJobData)
	})
}

// SystemSwitch creates a job to switch the key of a system from keyIDFrom to keyIDTo
// make sure the ctx provided has the tenant set.
// trigger can be KeyActionSetPrimary to indicate this switch is from a make primary key action
func (f *EventFactory) SystemSwitch(
	ctx context.Context,
	system *model.System,
	keyIDTo string,
	keyIDFrom string,
) (orbital.Job, error) {
	return f.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemSwitchJobData := SystemActionJobData{
			SystemID:  system.ID.String(),
			KeyIDTo:   keyIDTo,
			KeyIDFrom: keyIDFrom,
		}

		return f.createSystemEventJob(ctx, JobTypeSystemSwitch, systemSwitchJobData)
	})
}

// SystemSwitchNewPrimaryKey creates a job to switch the key of a system from keyIDFrom to keyIDTo
// make sure the ctx provided has the tenant set, triggered by a new primary key being set.
func (f *EventFactory) SystemSwitchNewPrimaryKey(
	ctx context.Context,
	system *model.System,
	keyIDTo string,
	keyIDFrom string,
) (orbital.Job, error) {
	return f.handleSystemStatus(ctx, system, func() (orbital.Job, error) {
		systemSwitchJobData := SystemActionJobData{
			SystemID:  system.ID.String(),
			KeyIDTo:   keyIDTo,
			KeyIDFrom: keyIDFrom,
		}

		return f.createSystemEventJob(ctx, JobTypeSystemSwitchNewPK, systemSwitchJobData)
	})
}

// KeyEnable creates a job to enable a key make sure the ctx provided has the tenant set.
func (f *EventFactory) KeyEnable(ctx context.Context, keyID string) (orbital.Job, error) {
	return f.createKeyEventJob(ctx, keyID, JobTypeKeyEnable)
}

// KeyDisable creates a job to disable a key make sure the ctx provided has the tenant set.
func (f *EventFactory) KeyDisable(ctx context.Context, keyID string) (orbital.Job, error) {
	return f.createKeyEventJob(ctx, keyID, JobTypeKeyDisable)
}

// KeyDetach creates a job to detach a key.
// Context provided must have the tenant set.
func (f *EventFactory) KeyDetach(ctx context.Context, keyID string) (orbital.Job, error) {
	return f.createKeyEventJob(ctx, keyID, JobTypeKeyDetach)
}

func (f *EventFactory) createKeyEventJob(
	ctx context.Context,
	keyID string,
	jobType JobType,
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
		Type:       jobType.String(),
		Data:       jobData,
	}

	return f.CreateJob(ctx, event)
}

func (f *EventFactory) createSystemEventJob(
	ctx context.Context,
	jobType JobType,
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
		Type:       jobType.String(),
		Data:       jobData,
	}

	return f.CreateJob(ctx, event)
}

func (f *EventFactory) handleSystemStatus(
	ctx context.Context,
	system *model.System,
	eventFn func() (orbital.Job, error),
) (orbital.Job, error) {
	job := orbital.Job{}
	previousStatus := system.Status

	err := f.repo.Transaction(ctx, func(ctx context.Context) error {
		if system.Status == cmkapi.SystemStatusPROCESSING {
			return ErrSystemProcessing
		}

		system.Status = cmkapi.SystemStatusPROCESSING

		_, err := f.repo.Patch(ctx, system, *repo.NewQuery().UpdateAll(true))
		if err != nil {
			return err
		}

		job, err = eventFn()

		return err
	})
	if err != nil {
		return job, err
	}

	err = f.repo.Set(ctx, &model.Event{
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

// dummyResolveTask here is only a dummy function so that we can create the orbital manager.
// The manager is not going to run, so no task resolution logic is needed.
func dummyResolveTask(
	_ context.Context,
	_ orbital.Job,
	_ orbital.TaskResolverCursor,
) (orbital.TaskResolverResult, error) {
	return orbital.CompleteTaskResolver(), nil
}
