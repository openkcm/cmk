package testutils

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
)

// InjectClientDataIntoContext adds identifier, groups to the context for testing.
func InjectClientDataIntoContext(ctx context.Context, identifier string, groups []string) context.Context {
	clientData := &auth.ClientData{
		Identifier: identifier,
		Groups:     groups,
	}
	ctx = context.WithValue(ctx, constants.ClientData, clientData)

	return ctx
}
