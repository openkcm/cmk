package manager

import (
	"context"
	"strings"

	"github.com/google/uuid"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

type SystemGroupFilter struct {
	ExpandSystems bool
	Pagination    repo.Pagination
}

type systemGroup struct {
	r repo.Repo
}

type SystemGroup interface {
	GetAllSystemGroups(
		ctx context.Context,
		filter SystemGroupFilter,
	) ([]*model.SystemGroup, int, error)
	UpdateSystemGroup(
		ctx context.Context,
		id uuid.UUID,
		systemGroupPatch cmkapi.SystemGroupPatch,
	) (*model.SystemGroup, error)
	DeleteAllSystemGroups(ctx context.Context) error
	CreateAllSystemGroups(ctx context.Context, supressWarning bool) error
	HandleNewSystem(
		ctx context.Context,
		system *model.System,
		systemGroupConfigGetter SystemGroupConfigGetter,
	) error
	HandleDeleteSystem(ctx context.Context, system *model.System) error
}

type SystemGroupConfigGetter interface {
	GetSystemGroupingConfig(ctx context.Context) (*model.SystemGroupConfig, error)
}

func NewSystemGroup(r repo.Repo) SystemGroup {
	return &systemGroup{
		r: r,
	}
}

func processSystemGroupName(sys *model.System) string {
	name, ok := sys.Properties["externalName"]
	if ok {
		return name
	}
	return sys.Identifier
}

func (s *systemGroup) GetAllSystemGroups(
	ctx context.Context,
	filter SystemGroupFilter,
) ([]*model.SystemGroup, int, error) {
	if !filter.ExpandSystems {
		return repo.ListAndCount(ctx, s.r, filter.Pagination, model.SystemGroup{}, repo.NewQuery())
	}

	systemGroups, count, err := repo.ListSystemGroupsAndSystem(ctx, s.r, filter.Pagination, repo.NewQuery())
	if err != nil {
		return nil, 0, err
	}

	return systemGroups, count, nil
}

func (s *systemGroup) UpdateSystemGroup(
	ctx context.Context,
	id uuid.UUID,
	systemGroupPatch cmkapi.SystemGroupPatch,
) (*model.SystemGroup, error) {
	systemGroup := &model.SystemGroup{
		ID: id,
	}
	_, err := s.r.First(
		ctx,
		systemGroup,
		*repo.NewQuery(),
	)
	if err != nil {
		return nil, err
	}

	if systemGroupPatch.Name != nil && strings.TrimSpace(*systemGroupPatch.Name) == "" {
		return nil, ErrNameCannotBeEmpty
	}

	if systemGroupPatch.Name != nil {
		systemGroup.Name = *systemGroupPatch.Name
	}

	if systemGroupPatch.Description != nil {
		systemGroup.Description = *systemGroupPatch.Description
	}

	if systemGroupPatch.SuppressWarning != nil {
		systemGroup.SuppressWarning = *systemGroupPatch.SuppressWarning
	}

	_, err = s.r.Patch(ctx, systemGroup, *repo.NewQuery())
	return systemGroup, err
}

func (s *systemGroup) HandleDeleteSystem(ctx context.Context, system *model.System) error {
	if system.SystemGroupID == nil {
		return nil
	}

	count, err := s.r.Count(ctx, &model.System{SystemGroupID: system.SystemGroupID}, *repo.NewQuery())
	if err != nil {
		return err
	}

	if count < 2 {
		_, err := s.r.Delete(ctx, &model.SystemGroup{ID: *system.SystemGroupID}, *repo.NewQuery())
		return err
	}
	return nil
}

func (s *systemGroup) HandleNewSystem(
	ctx context.Context,
	system *model.System,
	systemGroupConfigGetter SystemGroupConfigGetter,
) error {
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(repo.IdentifierField, system.Identifier),
		),
	)
	systems, _, err := repo.ListAndCountSystemWithProperties(ctx, s.r, repo.Pagination{}, query)
	if err != nil {
		return err
	}

	if len(systems) < 2 {
		return nil
	}
	var systemGroupID *uuid.UUID
	for _, sys := range systems {
		if systemGroupID != nil {
			break
		}
		systemGroupID = sys.SystemGroupID
	}

	if systemGroupID != nil {
		system.SystemGroupID = systemGroupID
		_, err := s.r.Patch(ctx, system, *repo.NewQuery())
		if err != nil {
			return err
		}
		return nil
	}

	return s.createSystemGroup(ctx, systemGroupConfigGetter, append(systems, system))
}

//nolint:funlen
func (s *systemGroup) CreateAllSystemGroups(ctx context.Context, supressWarning bool) error {
	type groupValue struct {
		name    string
		systems []*model.System
	}
	groupNames := make(map[string]groupValue)

	err := repo.ProcessInBatchWithOptions(
		ctx,
		s.r,
		repo.NewQuery(),
		repo.DefaultLimit,
		repo.BatchProcessOptions{},
		func(ctx context.Context, query repo.Query) ([]*model.System, error) {
			items, _, err := repo.ListAndCountSystemWithProperties(ctx, s.r, repo.Pagination{}, &query)
			return items, err
		},
		func(systems []*model.System) error {
			for _, sys := range systems {
				group, ok := groupNames[sys.Identifier]
				if !ok {
					groupNames[sys.Identifier] = groupValue{
						name:    processSystemGroupName(sys),
						systems: []*model.System{sys},
					}
					continue
				}
				if group.name == sys.Identifier {
					group.name = processSystemGroupName(sys)
				}
				group.systems = append(group.systems, sys)
				groupNames[sys.Identifier] = group
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	return s.r.Transaction(ctx, func(ctx context.Context) error {
		for _, group := range groupNames {
			if len(group.systems) < 2 {
				continue
			}

			groupID := uuid.New()
			err := s.r.Create(ctx, &model.SystemGroup{
				ID:              groupID,
				Name:            group.name,
				SuppressWarning: supressWarning,
			})
			if err != nil {
				return err
			}

			for _, sys := range group.systems {
				_, err := s.r.Patch(ctx, &model.System{ID: sys.ID, SystemGroupID: &groupID}, *repo.NewQuery())
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *systemGroup) DeleteAllSystemGroups(ctx context.Context) error {
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IDField, repo.NotNull)),
	)
	_, err := s.r.Delete(ctx, &model.SystemGroup{}, *query)
	return err
}

func (s *systemGroup) createSystemGroup(
	ctx context.Context,
	systemGroupConfigGetter SystemGroupConfigGetter,
	systems []*model.System,
) error {
	if len(systems) < 2 {
		return ErrInvalidSystemGroupSize
	}
	var name string
	for _, sys := range systems {
		name = processSystemGroupName(sys)
		if name != sys.Identifier {
			break
		}
	}

	return s.r.Transaction(ctx, func(ctx context.Context) error {
		systemGroupingConfig, err := systemGroupConfigGetter.GetSystemGroupingConfig(ctx)
		if err != nil {
			return err
		}
		groupID := uuid.New()
		err = s.r.Create(ctx, model.SystemGroup{
			ID:              groupID,
			Name:            name,
			SuppressWarning: !systemGroupingConfig.WarnCoverage,
		})
		if err != nil {
			return err
		}

		for _, sys := range systems {
			sys.SystemGroupID = &groupID
			_, err := s.r.Patch(ctx, sys, *repo.NewQuery())
			if err != nil {
				return err
			}
		}
		return nil
	})
}
