package testutils

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/common-sdk/pkg/auth"
)

type AuthClientData struct {
	Group      *model.Group
	Identifier string
}

type ClientMapOpt func(*auth.ClientData)

func (cd AuthClientData) GetClientMap(opts ...ClientMapOpt) map[any]any {
	clientData := getClientGroupsData(cd.Identifier, []string{cd.Group.IAMIdentifier})

	for _, o := range opts {
		o(clientData)
	}

	return map[any]any{constants.ClientData: clientData}
}

type AuthClientOpt func(*model.Group)

func NewAuthClient(ctx context.Context, tb testing.TB, r repo.Repo, opts ...AuthClientOpt) AuthClientData {
	group := NewGroup(func(g *model.Group) {
		g.ID = uuid.New()
		g.Name = uuid.NewString()
		g.IAMIdentifier = uuid.NewString()
		g.Role = constants.TenantAuditorRole
	})

	for _, o := range opts {
		o(group)
	}

	CreateTestEntities(ctx, tb, r, group)

	return AuthClientData{
		Group:      group,
		Identifier: uuid.NewString(),
	}
}

func WithAuditorRole() AuthClientOpt {
	return func(g *model.Group) {
		g.Role = constants.TenantAuditorRole
	}
}

func WithKeyAdminRole() AuthClientOpt {
	return func(g *model.Group) {
		g.Role = constants.KeyAdminRole
	}
}

func WithTenantAdminRole() AuthClientOpt {
	return func(g *model.Group) {
		g.Role = constants.TenantAdminRole
	}
}

func InitialiseKeyConfig(authClient AuthClientData) KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *authClient.Group
	}
}

func GetClientGroupsMap(identifier string, groupNames []string) map[any]any {
	return map[any]any{constants.ClientData: getClientGroupsData(identifier, groupNames)}
}

func getClientGroupsData(identifier string, groupNames []string) *auth.ClientData {
	return &auth.ClientData{
		Identifier: identifier,
		Groups:     groupNames,
	}
}
