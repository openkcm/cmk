package tasks

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
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

	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskKeystorePoolRole)
	if err != nil {
		log.Error(ctx, "failed to fill keystore pool", err)
		return err
	}

	err = k.updater.FillKeystorePool(ctx, k.size)
	if err != nil {
		log.Error(ctx, "failed to fill keystore pool", err)
		return err
	}

	return nil
}

func (k *KeystorePoolFiller) TaskType() string {
	return config.TypeKeystorePool
}
