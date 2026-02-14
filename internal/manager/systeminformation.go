package manager

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/openkcm/plugin-sdk/api/service/systeminformation"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

var (
	ErrGettingSystemList = errors.New("error getting system list")
	ErrUpdatingSystem    = errors.New("error updating system")
	ErrNoPluginInCatalog = errors.New("no plugin in catalog")
	ErrNoSystem          = errors.New("no system found")
)

type SystemInformation struct {
	repo      repo.Repo
	service   systeminformation.SystemInformation
	systemCfg *config.System
}

func NewSystemInformationManager(repo repo.Repo,
	service systeminformation.SystemInformation, systemCfg *config.System,
) *SystemInformation {
	return &SystemInformation{
		service:   service,
		repo:      repo,
		systemCfg: systemCfg,
	}
}

func (si *SystemInformation) UpdateSystems(ctx context.Context) error {
	systems := []*model.System{}

	err := si.repo.List(ctx, model.System{}, &systems, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrGettingSystemList, err)
	}

	for _, sys := range systems {
		err = si.updateSystem(ctx, sys)
		if err != nil {
			return err
		}
	}

	return nil
}

func (si *SystemInformation) UpdateSystemByExternalID(ctx context.Context, externalID string) error {
	sys := &model.System{Identifier: externalID}

	_, err := si.repo.First(ctx, sys, *repo.NewQuery())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.Wrap(ErrNoSystem, err)
		}

		return errs.Wrap(ErrGettingSystem, err)
	}

	return si.updateSystem(ctx, sys)
}

func (si *SystemInformation) updateSystem(ctx context.Context, system *model.System) error {
	ctx = model.LogInjectSystem(ctx, system)

	log.Debug(ctx, "Requesting SIS for properties")

	resp, err := si.service.GetSystemInfo(ctx, &systeminformation.GetSystemInfoRequest{
		ID:   system.Identifier,
		Type: strings.ToLower(system.Type),
	})
	if err != nil {
		log.Warn(ctx, "Could not get information from SIS", log.ErrorAttr(err))
		return nil
	}

	metadata := resp.Metadata
	if metadata == nil {
		log.Warn(ctx, "No system information from SIS")
		return nil
	}

	log.Debug(ctx, "SIS Response", slog.Any("SIS Response", metadata))

	system, err = repo.GetSystemByIDWithProperties(ctx, si.repo, system.ID, repo.NewQuery())
	if err != nil {
		return errs.Wrap(err, repo.ErrSystemProperties)
	}

	updated := system.UpdateSystemProperties(metadata, si.systemCfg)
	if updated {
		log.Debug(ctx, "Update System with SIS Information", slog.Any("sisSystem", *system))

		_, err := si.repo.Patch(ctx, system, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrUpdatingSystem, err)
		}
	}

	return nil
}
