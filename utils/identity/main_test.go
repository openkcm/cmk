package identity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	"github.com/openkcm/cmk/utils/identity"
)

func TestGetUserName(t *testing.T) {
	idm := testplugins.NewTestIdentityManagement()

	t.Run("Should return unknown on not found", func(t *testing.T) {
		ctx := testutils.InjectBusinessUserDataIntoContext(t.Context(), "a", []string{"a"})
		user, err := identity.GetUserName(ctx, idm, "1")
		assert.NoError(t, err)
		assert.Equal(t, constants.UnknownUserName, user)
	})

	t.Run("Should return user", func(t *testing.T) {
		ctx := testutils.InjectBusinessUserDataIntoContext(t.Context(), "a", []string{"a"})
		idmUser := identitymanagement.User{
			Email: uuid.NewString(),
			ID:    uuid.NewString(),
		}
		idm.PutUser(idmUser)
		user, err := identity.GetUserName(ctx, idm, idmUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, idmUser.Email, user)
	})
}
