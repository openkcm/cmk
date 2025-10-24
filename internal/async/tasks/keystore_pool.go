package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/repo"
)

type KeystorePoolUpdater interface {
	FillKeystorePool(ctx context.Context, size int) error
}

type KeystorePoolFiller struct {
	updater KeystorePoolUpdater
	size    int
	repo    repo.Repo
}

func NewKeystorePoolFiller(
	updater KeystorePoolUpdater,
	repo repo.Repo,
	poolConfig config.KeystorePool,
) *KeystorePoolFiller {
	return &KeystorePoolFiller{
		updater: updater,
		repo:    repo,
		size:    poolConfig.Size,
	}
}

func (k *KeystorePoolFiller) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	log.Info(ctx, "Starting Keystore Pool Filler Task")

	err := k.updater.FillKeystorePool(ctx, k.size)
	if err != nil {
		log.Error(ctx, "failed to fill keystore pool", err)
		return err
	}

	return nil
}

func (k *KeystorePoolFiller) TaskType() string {
	return config.TypeKeystorePool
}
