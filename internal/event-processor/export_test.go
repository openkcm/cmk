package eventprocessor

import (
	"context"

	"github.com/openkcm/orbital"

	"github.com/openkcm/cmk/internal/auditor"
)

func (c *CryptoReconciler) ConfirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmerResult, error) {
	return c.confirmJob(ctx, job)
}

func (c *CryptoReconciler) JobDoneFunc(ctx context.Context, job orbital.Job) error {
	return c.jobDoneFunc(ctx, job)
}

func (c *CryptoReconciler) JobFailedFunc(ctx context.Context, job orbital.Job) error {
	return c.jobFailedFunc(ctx, job)
}

func (c *CryptoReconciler) JobCanceledFunc(ctx context.Context, job orbital.Job) error {
	return c.jobCanceledFunc(ctx, job)
}

func (c *CryptoReconciler) ResolveTasks() orbital.TaskResolveFunc {
	return c.resolveTasks()
}

//nolint:forcetypeassert
func (c *CryptoReconciler) DisableAuditLog() {
	c.jobHandlerMap[JobTypeSystemLink].(*SystemLinkJobHandler).cmkAuditor = &auditor.Auditor{}
	c.jobHandlerMap[JobTypeSystemUnlink].(*SystemUnlinkJobHandler).cmkAuditor = &auditor.Auditor{}
	c.jobHandlerMap[JobTypeSystemSwitch].(*SystemSwitchJobHandler).cmkAuditor = &auditor.Auditor{}
	c.jobHandlerMap[JobTypeSystemKeyRotate].(*SystemKeyRotateJobHandler).cmkAuditor = &auditor.Auditor{}
}
