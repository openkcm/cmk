package eventprocessor

import (
	"context"
	"fmt"

	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func getSystemByID(ctx context.Context, r repo.Repo, systemID string) (*model.System, error) {
	var system model.System

	ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := r.First(ctx, &system, *query)
	if err != nil {
		return nil, err
	}

	return &system, nil
}

func getKeyByKeyID(ctx context.Context, r repo.Repo, keyID string) (*model.Key, error) {
	var key model.Key

	ck := repo.NewCompositeKey().Where(repo.IDField, keyID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := r.First(ctx, &key, *query)
	if err != nil {
		return nil, fmt.Errorf("failed to get key by ID %s: %w", keyID, err)
	}

	return &key, nil
}

func getTenantByID(ctx context.Context, r repo.Repo, tenantID string) (*model.Tenant, error) {
	cmkContext := cmkcontext.CreateTenantContext(ctx, tenantID)

	var tenant model.Tenant

	_, err := r.First(cmkContext, &tenant, *repo.NewQuery().
		Where(
			repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().
					Where(repo.IDField, tenantID),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

func updateSystem(ctx context.Context, r repo.Repo, system *model.System) error {
	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).UpdateAll(true)

	updated, err := r.Patch(ctx, system, *query)
	if err != nil {
		return fmt.Errorf("failed to update system %s: %w", system.ID, err)
	}

	if !updated {
		log.Warn(ctx, fmt.Sprintf("system with ID %s was not updated", system.ID))
	}

	return nil
}

func updateKey(ctx context.Context, r repo.Repo, key *model.Key) error {
	ck := repo.NewCompositeKey().Where(repo.IDField, key.ID)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).UpdateAll(true)

	updated, err := r.Patch(ctx, key, *query)
	if err != nil {
		return fmt.Errorf("failed to update key %s: %w", key.ID, err)
	}

	if !updated {
		log.Warn(ctx, fmt.Sprintf("key with ID %s was not updated", key.ID))
	}

	return nil
}
