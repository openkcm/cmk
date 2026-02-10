package testutils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	AuditorName      = "auditor"
	AuditorGroupName = "auditors"
	AuditorGroupID   = "7a3834b8-1e41-4adc-bda2-84c72ad1d871"
	AuditorGroupUUID = uuid.MustParse(AuditorGroupID)

	TenantAdminName      = "tenantadmin"
	TenantAdminGroupName = "tenantadmins"
	TenantAdminGroupID   = "7a3834b8-1e41-4adc-bda2-84c72ad1d562"
	TenantAdminGroupUUID = uuid.MustParse(TenantAdminGroupID)

	KeyAdminName      = "keyadmin"
	KeyAdminGroupName = "keyadmins"
	KeyAdminGroupID   = "7a3834b8-1e41-4adc-bda2-73c72ad1d560"
	KeyAdminGroupUUID = uuid.MustParse(KeyAdminGroupID)
)

// For all Roles

func CreateRoleGroups(ctx context.Context, tb testing.TB,
	r repo.Repo) (*model.Group, *model.Group, *model.Group) {
	tb.Helper()

	auditorGroup := GetAuditorGroup()
	tenantAdminGroup := GetTenantAdminGroup()
	keyAdminGroup := GetKeyAdminGroup()
	CreateTestEntities(ctx, tb, r, auditorGroup, tenantAdminGroup, keyAdminGroup)
	return auditorGroup, tenantAdminGroup, keyAdminGroup
}

// For auditor Roles

func CreateAuditorGroup(ctx context.Context, tb testing.TB, r repo.Repo) *model.Group {
	tb.Helper()

	return CreateOtherAuditorGroup(ctx, tb, r, AuditorGroupName, ptr.PointTo(AuditorGroupID))
}

// CreateOtherAuditorGroup: When we don't want to use the provided group name. Unusual for auditor, since there is
// only a single auditor group allowed per tenant but we may want to test the illegitimate case
func CreateOtherAuditorGroup(ctx context.Context, tb testing.TB, r repo.Repo, name string, id *string) *model.Group {
	tb.Helper()

	group := GetOtherAuditorGroup(name, id)
	CreateTestEntities(ctx, tb, r, group)
	return group
}

func GetAuditorGroup() *model.Group {
	return GetOtherAuditorGroup(AuditorGroupName, ptr.PointTo(AuditorGroupID))
}

func GetOtherAuditorGroup(name string, id *string) *model.Group {
	idStr := uuid.NewString()
	if id != nil {
		idStr = *id
	}
	return NewGroup(func(g *model.Group) {
		g.ID = uuid.MustParse(idStr)
		g.Name = name
		g.IAMIdentifier = name
		g.Role = constants.TenantAuditorRole
	})
}

func WithAuditorGroup() KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *GetAuditorGroup()
	}
}

func GetAuditorClientData() *auth.ClientData {
	return GetClientGroupData(AuditorName, AuditorGroupName)
}

func GetAuditorClientMap() map[any]any {
	return GetClientGroupMap(AuditorName, AuditorGroupName)
}

// For tenant admin Roles

func CreateTenantAdminGroup(ctx context.Context, tb testing.TB, r repo.Repo) *model.Group {
	tb.Helper()

	return CreateOtherTenantAdminGroup(ctx, tb, r, TenantAdminGroupName, ptr.PointTo(TenantAdminGroupID))
}

// CreateOtherTenantAdminGroup: When we don't want to use the provided group name. Unusual for tenant admin,
// since there is only a single tenant admin group allowed per tenant but we may want to test the illegitimate case
func CreateOtherTenantAdminGroup(ctx context.Context, tb testing.TB, r repo.Repo,
	name string, id *string) *model.Group {
	tb.Helper()

	group := GetOtherTenantAdminGroup(name, id)
	CreateTestEntities(ctx, tb, r, group)
	return group
}

func GetTenantAdminGroup() *model.Group {
	return GetOtherTenantAdminGroup(TenantAdminGroupName, ptr.PointTo(TenantAdminGroupID))
}

func GetOtherTenantAdminGroup(name string, id *string) *model.Group {
	idStr := uuid.NewString()
	if id != nil {
		idStr = *id
	}
	return NewGroup(func(g *model.Group) {
		g.ID = uuid.MustParse(idStr)
		g.Name = name
		g.IAMIdentifier = name
		g.Role = constants.TenantAdminRole
	})
}

func WithTenantAdminGroup() KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *GetTenantAdminGroup()
	}
}

func GetTenantAdminClientData() *auth.ClientData {
	return GetClientGroupData(TenantAdminName, TenantAdminGroupName)
}

func GetTenantAdminClientMap() map[any]any {
	return GetClientGroupMap(TenantAdminName, TenantAdminGroupName)
}

// For key admin Roles

func CreateKeyAdminGroup(ctx context.Context, tb testing.TB, r repo.Repo) *model.Group {
	tb.Helper()

	return CreateOtherKeyAdminGroup(ctx, tb, r, KeyAdminGroupName, ptr.PointTo(KeyAdminGroupID))
}

// CreateOtherKeyAdminGroup: When we don't want to use the provided group name.
func CreateOtherKeyAdminGroup(ctx context.Context, tb testing.TB, r repo.Repo, name string, id *string) *model.Group {
	tb.Helper()

	group := GetOtherKeyAdminGroup(name, id)
	CreateTestEntities(ctx, tb, r, group)
	return group
}

func GetKeyAdminGroup() *model.Group {
	return GetOtherKeyAdminGroup(KeyAdminGroupName, ptr.PointTo(KeyAdminGroupID))
}

func GetOtherKeyAdminGroup(name string, id *string) *model.Group {
	idStr := uuid.NewString()
	if id != nil {
		idStr = *id
	}
	return NewGroup(func(g *model.Group) {
		g.ID = uuid.MustParse(idStr)
		g.Name = name
		g.IAMIdentifier = name
		g.Role = constants.KeyAdminRole
	})
}

func WithKeyAdminGroup() KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *GetKeyAdminGroup()
	}
}

func GetKeyAdminClientData() *auth.ClientData {
	return GetClientGroupData(KeyAdminName, KeyAdminGroupName)
}

func GetKeyAdminClientMap() map[any]any {
	return GetClientGroupMap(KeyAdminName, KeyAdminGroupName)
}

// Common

func GetClientGroupData(identifier, groupName string) *auth.ClientData {
	return GetClientGroupsData(identifier, []string{groupName})
}

func GetClientGroupMap(identifier, groupName string) map[any]any {
	return map[any]any{constants.ClientData: GetClientGroupData(identifier, groupName)}
}

func GetClientGroupsData(identifier string, groupNames []string) *auth.ClientData {
	return &auth.ClientData{
		Identifier: identifier,
		Groups:     groupNames,
	}
}

func GetClientGroupsMap(identifier string, groupNames []string) map[any]any {
	return map[any]any{constants.ClientData: GetClientGroupsData(identifier, groupNames)}
}

func GetClientNoGroupsMap(identifier string) map[any]any {
	return map[any]any{constants.ClientData: GetClientGroupsData(identifier, []string{})}
}
