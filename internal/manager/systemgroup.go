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
}

func NewSystemGroup(r repo.Repo) SystemGroup {
	return &systemGroup{
		r: r,
	}
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
