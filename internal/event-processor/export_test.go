package eventprocessor

import (
	"context"

	"github.com/openkcm/orbital"
)

func (c *CryptoReconciler) JobTerminationFunc(ctx context.Context, job orbital.Job) error {
	return c.jobTerminationFunc(ctx, job)
}

func (c *CryptoReconciler) ResolveTasks() orbital.TaskResolveFunc {
	return c.resolveTasks()
}
