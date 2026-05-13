//nolint:testpackage
package testutils

import (
	"strconv"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
)

func TestAuthClientData_GetClientMapAppliesOptions(t *testing.T) {
	group := NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "group-a"
	})
	authClient := AuthClientData{
		Group:      group,
		GroupID:    group.ID.String(),
		Identifier: "user-a",
	}

	clientMap := authClient.GetClientMap(
		WithAdditionalGroup("group-b"),
		WithOverriddenIdentifier("user-b"),
	)

	clientData, ok := clientMap[constants.ClientData].(*auth.ClientData)
	require.True(t, ok)
	require.NotNil(t, clientData)
	assert.Equal(t, "user-b", clientData.Identifier)
	assert.Equal(t, []string{"group-a", "group-b"}, clientData.Groups)
}

func TestWithOverriddenGroupGeneratesRequestedCount(t *testing.T) {
	authClient := AuthClientData{
		Group: NewGroup(func(g *model.Group) {
			g.IAMIdentifier = "seed-group"
		}),
		Identifier: "seed-user",
	}

	clientMap := authClient.GetClientMap(WithOverriddenGroup(3))
	clientData, ok := clientMap[constants.ClientData].(*auth.ClientData)
	require.True(t, ok)
	require.NotNil(t, clientData)
	require.Len(t, clientData.Groups, 3)

	for _, g := range clientData.Groups {
		assert.NotEmpty(t, g)
	}
}

func TestAuthClientOptionsAndFactory(t *testing.T) {
	t.Run("roles", func(t *testing.T) {
		auditor := newAuthClient(WithAuditorRole())
		assert.Equal(t, constants.TenantAuditorRole, auditor.Group.Role)

		keyAdmin := newAuthClient(WithKeyAdminRole())
		assert.Equal(t, constants.KeyAdminRole, keyAdmin.Group.Role)

		tenantAdmin := newAuthClient(WithTenantAdminRole())
		assert.Equal(t, constants.TenantAdminRole, tenantAdmin.Group.Role)
	})

	t.Run("identifier", func(t *testing.T) {
		custom := newAuthClient(WithIdentifier("custom-id"))
		assert.Equal(t, "custom-id", custom.Identifier)
	})
}

func TestWithAuthClientDataKC(t *testing.T) {
	authClient := newAuthClient(WithTenantAdminRole())
	kc := &model.KeyConfiguration{}

	WithAuthClientDataKC(authClient)(kc)

	assert.Equal(t, authClient.Group.ID, kc.AdminGroupID)
	assert.Equal(t, authClient.Group.ID, kc.AdminGroup.ID)
	assert.Equal(t, authClient.Group.IAMIdentifier, kc.AdminGroup.IAMIdentifier)
}

func TestClientMapHelpers(t *testing.T) {
	clientMap := GetClientMap("id-a", []string{"g1", "g2"})
	clientData, ok := clientMap[constants.ClientData].(*auth.ClientData)
	require.True(t, ok)
	require.NotNil(t, clientData)
	assert.Equal(t, "id-a", clientData.Identifier)
	assert.Equal(t, []string{"g1", "g2"}, clientData.Groups)

	groupless := GetGrouplessClientMap()
	grouplessData, ok := groupless[constants.ClientData].(*auth.ClientData)
	require.True(t, ok)
	require.NotNil(t, grouplessData)
	assert.Empty(t, grouplessData.Groups)
	assert.NotEmpty(t, grouplessData.Identifier)

	invalid := GetInvalidClientMap()
	invalidData, ok := invalid[constants.ClientData].(*auth.ClientData)
	require.True(t, ok)
	require.NotNil(t, invalidData)
	assert.NotEmpty(t, invalidData.Identifier)
	assert.Len(t, invalidData.Groups, 2)
}

func TestNewSignedClientDataHeadersMutatesAndSigns(t *testing.T) {
	privateKey, _, err := GenerateTestKeyPair()
	require.NoError(t, err)

	clientData := &auth.ClientData{
		Identifier: "user-2",
		Groups:     []string{"g1"},
	}

	headers := NewSignedClientDataHeaders(t, clientData, privateKey, 2)
	require.NotEmpty(t, headers.Get(auth.HeaderClientData))
	require.NotEmpty(t, headers.Get(auth.HeaderClientDataSignature))
	assert.Equal(t, strconv.Itoa(2), clientData.KeyID)
	assert.Equal(t, auth.SignatureAlgorithmRS256, clientData.SignatureAlgorithm)
}
