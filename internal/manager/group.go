package manager

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrGroupRole = errors.New("unsupported role for group creation")

type Group interface {
	GetGroups(ctx context.Context, skip int, top int) ([]*model.Group, int, error)
	CreateGroup(ctx context.Context, group *model.Group) (*model.Group, error)
	DeleteGroupByID(ctx context.Context, id uuid.UUID) error
	GetGroupByID(ctx context.Context, id uuid.UUID) (*model.Group, error)
	UpdateGroup(ctx context.Context, id uuid.UUID, patchGroup cmkapi.GroupPatch) (*model.Group, error)
}

type GroupManager struct {
	repo repo.Repo
}

func NewGroupManager(
	repository repo.Repo,
) *GroupManager {
	return &GroupManager{
		repo: repository,
	}
}

func (m *GroupManager) GetGroups(ctx context.Context, skip int, top int) ([]*model.Group, int, error) {
	var groups []*model.Group

	count, err := m.repo.List(ctx, model.Group{}, &groups, *repo.NewQuery().
		SetLimit(top).
		SetOffset(skip),
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrListGroups, err)
	}

	return groups, count, nil
}

func (m *GroupManager) CreateGroup(ctx context.Context, group *model.Group) (*model.Group, error) {
	if !m.isSupportedRole(group) {
		return nil, ErrGroupRole
	}

	err := m.repo.Create(ctx, group)
	if err != nil {
		return nil, errs.Wrap(ErrCreateGroups, err)
	}

	return group, nil
}

func (m *GroupManager) DeleteGroupByID(ctx context.Context, id uuid.UUID) error {
	group, err := m.GetGroupByID(ctx, id)
	if err != nil {
		return errs.Wrap(ErrDeleteGroups, err)
	}

	if m.isMandatoryGroup(group) {
		return ErrInvalidGroupDelete
	}

	keyConfig := &model.KeyConfiguration{}
	exist, err := m.repo.First(
		ctx,
		keyConfig,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(
				repo.NewCompositeKey().Where(repo.AdminGroupIDField, id))),
	)

	if exist {
		return ErrInvalidGroupDelete
	}

	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return errs.Wrap(ErrGetGroups, err)
	}

	_, err = m.repo.Delete(ctx, &model.Group{ID: id}, *repo.NewQuery())
	if err != nil {
		return errs.Wrap(ErrDeleteGroups, err)
	}

	return nil
}

func (m *GroupManager) GetGroupByID(ctx context.Context, id uuid.UUID) (*model.Group, error) {
	group := &model.Group{ID: id}

	_, err := m.repo.First(ctx, group, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGetGroups, err)
	}

	return group, nil
}

func (m *GroupManager) UpdateGroup(
	ctx context.Context,
	id uuid.UUID,
	patchGroup cmkapi.GroupPatch,
) (*model.Group, error) {
	group, err := m.GetGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !ptr.IsValidStrPtr(patchGroup.Name) {
		return nil, errs.Wrap(ErrNameCannotBeEmpty, nil)
	}

	if m.isMandatoryGroup(group) || m.isReservedName(patchGroup) {
		return nil, ErrInvalidGroupRename
	}

	if patchGroup.Name != nil {
		group.Name = *patchGroup.Name

		tenantID, err := cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return nil, err
		}

		group.IAMIdentifier = model.NewIAMIdentifier(group.Name, tenantID)
	}

	if patchGroup.Description != nil {
		group.Description = *patchGroup.Description
	}

	_, err = m.repo.Patch(ctx, group, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrUpdateGroups, err)
	}

	return group, nil
}

func (m *GroupManager) isMandatoryGroup(group *model.Group) bool {
	return group.Name == constants.TenantAdminGroup || group.Name == constants.TenantAuditorGroup
}

func (m *GroupManager) isReservedName(patchGroup cmkapi.GroupPatch) bool {
	if patchGroup.Name == nil {
		return false
	}

	return m.isMandatoryGroup(&model.Group{Name: *patchGroup.Name})
}

func (m *GroupManager) isSupportedRole(group *model.Group) bool {
	switch group.Role {
	case constants.TenantAdminRole, constants.TenantAuditorRole, constants.KeyAdminRole:
		return true
	default:
		return false
	}
}
