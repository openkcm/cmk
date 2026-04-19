package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
)

type KeystorePoolFiller interface {
	FillKeystorePool(ctx context.Context, size int) error
}

type keystorePoolFiller struct {
	updater KeystorePoolFiller
	size    int
	repo    repo.Repo
}

func NewKeystorePoolFiller(
	updater KeystorePoolFiller,
	repo repo.Repo,
	poolConfig config.KeystorePool,
) async.TaskHandler {
	return &keystorePoolFiller{
		updater: updater,
		repo:    repo,
		size:    poolConfig.Size,
	}
}

func (k *keystorePoolFiller) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	log.Info(ctx, "Starting Keystore Pool Filler Task")

	err := k.updater.FillKeystorePool(ctx, k.size)
	if err != nil {
		log.Error(ctx, "failed to fill keystore pool", err)
		return err
	}

	return nil
}

func (k *keystorePoolFiller) TaskType() string {
	return config.TypeKeystorePool
}
