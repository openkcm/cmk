package testutils

import (
	"context"

	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
)

// InjectBusinessUserDataIntoContext adds identifier, groups to the context for testing.
func InjectBusinessUserDataIntoContext(ctx context.Context, identifier string, groups []string) context.Context {
	businessUserData := &auth.ClientData{
		Identifier: identifier,
		Groups:     groups,
	}
	ctx = context.WithValue(ctx, constants.UserType, constants.BusinessUser)
	ctx = context.WithValue(ctx, constants.BusinessUserData, businessUserData)

	return ctx
}
