package manager

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo"
)

var (
	ErrSisPlugin         = errors.New("system information plugin error")
	ErrGettingSystemList = errors.New("error getting system list")
	ErrUpdatingSystem    = errors.New("error updating system")
	ErrNoPluginInCatalog = errors.New("no plugin in catalog")
	ErrNoSystem          = errors.New("no system found")
)

const (
	pluginName = "SYSINFO"
)

type SystemInformation struct {
	repo      repo.Repo
	sisClient systeminformationv1.SystemInformationServiceClient
	systemCfg *config.System
}

func NewSystemInformationManager(repo repo.Repo,
	catalog *cmkplugincatalog.Registry, systemCfg *config.System,
) (*SystemInformation, error) {
	client, err := createClient(catalog)
	if err != nil {
		return nil, err
	}

	return &SystemInformation{
		sisClient: client,
		repo:      repo,
		systemCfg: systemCfg,
	}, nil
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

	resp, err := si.sisClient.Get(ctx, &systeminformationv1.GetRequest{
		Id:   system.Identifier,
		Type: strings.ToLower(system.Type),
	})
	if err != nil {
		log.Warn(ctx, "Could not get information from SIS", log.ErrorAttr(err))
		return nil
	}

	metadata := resp.GetMetadata()
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

func createClient(catalog *cmkplugincatalog.Registry) (systeminformationv1.SystemInformationServiceClient, error) {
	//nolint: staticcheck
	systemInformation := catalog.LookupByTypeAndName(systeminformationv1.Type, pluginName)
	if systemInformation == nil {
		return nil, ErrNoPluginInCatalog
	}

	return systeminformationv1.NewSystemInformationServiceClient(systemInformation.ClientConnection()), nil
}
