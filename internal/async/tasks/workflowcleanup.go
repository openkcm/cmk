package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

type WorkflowRemoval interface {
	CleanupTerminalWorkflows(ctx context.Context) error
}

type WorkflowCleaner struct {
	workflowRemoval WorkflowRemoval
	repo            repo.Repo
}

func NewWorkflowCleaner(
	workflowRemoval WorkflowRemoval,
	repo repo.Repo,
) *WorkflowCleaner {
	return &WorkflowCleaner{
		workflowRemoval: workflowRemoval,
		repo:            repo,
	}
}

func (wc *WorkflowCleaner) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	var tenants []*model.Tenant

	_, err := wc.repo.List(ctx, model.Tenant{}, &tenants, *repo.NewQuery())
	if err != nil {
		log.Error(ctx, "Getting tenants for Workflow Cleanup", err)
		return nil
	}

	for _, tenant := range tenants {
		ctx := log.InjectTenant(cmkcontext.CreateTenantContext(ctx, tenant.ID), tenant)
		log.Debug(ctx, "Cleaning up expired workflows")

		err = wc.workflowRemoval.CleanupTerminalWorkflows(ctx)
		if err != nil {
			log.Error(ctx, "Running Workflow Cleanup", err)
		}
	}

	return nil
}

func (wc *WorkflowCleaner) TaskType() string {
	return config.TypeWorkflowCleanup
}
