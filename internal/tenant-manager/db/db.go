package db

import (
	"context"
	"errors"
	"log/slog"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/namespace"
	"github.com/google/uuid"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	sqlRepo "github.com/openkcm/cmk-core/internal/repo/sql"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

var _ slogctx.Handler

const (
	msgBuildingIAMIdentifier = "building IAM identifier"
)

func CreateSchema(ctx context.Context, db *multitenancy.DB, tenant *model.Tenant) error {
	err := Validate(tenant.SchemaName)
	if err != nil {
		return errs.Wrap(ErrValidatingSchema, err)
	}

	r := sqlRepo.NewRepository(db)
	err = r.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		log.Info(ctx, "Creating tenant")

		err = r.Create(ctx, tenant)
		if err != nil {
			if errors.Is(err, repo.ErrUniqueConstraint) {
				err = errs.Wrap(ErrOnboardingInProgress, err)
			}

			return errs.Wrap(ErrCreatingTenant, err)
		}

		return migrate(ctx, db, tenant)
	})

	return err
}

func migrate(ctx context.Context, db *multitenancy.DB, tenant *model.Tenant) error {
	err := db.MigrateTenantModels(ctx, tenant.SchemaName)
	log.Info(ctx, "Migrating tenant models")

	if err != nil {
		return errs.Wrap(ErrMigratingTenantModels, err)
	}

	return nil
}

// CreateDefaultGroups CreateGroups creates the default admin and auditor groups for a tenant.
func CreateDefaultGroups(ctx context.Context, tenant *model.Tenant, r repo.Repo) error {
	groupCtx := cmkcontext.CreateTenantContext(ctx, tenant.ID)

	iamAdmin, err := BuildIAMIdentifier(constants.TenantAdminGroup, tenant.ID)
	if err != nil {
		log.Error(groupCtx, msgBuildingIAMIdentifier, err)
		return err
	}

	iamAuditor, err := BuildIAMIdentifier(constants.TenantAuditorGroup, tenant.ID)
	if err != nil {
		log.Error(groupCtx, msgBuildingIAMIdentifier, err)
		return err
	}

	err = r.Transaction(groupCtx, func(ctx context.Context, r repo.Repo) error {
		err = r.Create(ctx, &model.Group{
			ID:            uuid.New(),
			Name:          constants.TenantAdminGroup,
			Role:          constants.TenantAdminRole,
			IAMIdentifier: iamAdmin,
		})
		if err != nil {
			if errors.Is(err, repo.ErrUniqueConstraint) {
				err = errs.Wrap(ErrOnboardingInProgress, err)
			}

			log.Error(ctx, "Error creating group", err, slog.String("Group", constants.TenantAdminGroup))

			return errs.Wrap(ErrCreatingGroups, err)
		}

		err = r.Create(ctx, &model.Group{
			ID:            uuid.New(),
			Name:          constants.TenantAuditorGroup,
			Role:          constants.TenantAuditorRole,
			IAMIdentifier: iamAuditor,
		})
		if err != nil {
			if errors.Is(err, repo.ErrUniqueConstraint) {
				err = errs.Wrap(ErrOnboardingInProgress, err)
			}

			log.Error(ctx, "Error creating group", err, slog.String("Group", constants.TenantAuditorGroup))

			return errs.Wrap(ErrCreatingGroups, err)
		}

		return nil
	})

	return err
}

func BuildIAMIdentifier(groupType, tenantID string) (string, error) {
	if tenantID == "" {
		return "", ErrEmptyTenantID
	}

	if groupType != constants.TenantAdminGroup && groupType != constants.TenantAuditorGroup {
		return "", ErrInvalidGroupType
	}

	return model.NewIAMIdentifier(groupType, tenantID), nil
}

func Validate(schema string) error {
	err := namespace.Validate(schema)
	if err != nil {
		return err
	}

	if len(schema) < 3 || len(schema) > 63 {
		return ErrSchemaNameLength
	}

	return nil
}
