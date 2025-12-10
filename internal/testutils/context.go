package testutils

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/auth"

	"github.tools.sap/kms/cmk/internal/constants"
)

// InjectClientDataIntoContext adds identifier, groups to the context for testing.
func InjectClientDataIntoContext(ctx context.Context, identifier string, groups []string) context.Context {
	userGroups := make([]constants.UserGroup, len(groups))
	for i, g := range groups {
		userGroups[i] = constants.UserGroup(g)
	}

	clientData := &auth.ClientData{
		Identifier: identifier,
		Groups:     groups,
	}
	ctx = context.WithValue(ctx, constants.ClientData, clientData)

	return ctx
}
