package testutils

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// AuthClientData contains a group and an identifier associated with an AuthClient
type AuthClientData struct {
	Group      *model.Group
	GroupID    string // For convenience. Just a string version of the Group.ID
	Identifier string
}

// ClientMapOpt are options which can be used, for example, when retrieving the
// ClientData from an AuthClient
type ClientMapOpt func(*auth.ClientData)

// NewAuthClient creates an AuthClient using random strings for values and creates
// the group in the database
func NewAuthClient(ctx context.Context, tb testing.TB, r repo.Repo, opts ...AuthClientOpt) AuthClientData {
	tb.Helper()
	authClient := newAuthClient(opts...)
	CreateTestEntities(ctx, tb, r, authClient.Group)
	return authClient
}

// GetClientMap gets the ClientMap from the AuthClient. This can be used to authenticate
func (cd AuthClientData) GetClientMap(opts ...ClientMapOpt) map[any]any {
	clientData := getClientData(cd.Identifier, []string{cd.Group.IAMIdentifier})

	for _, o := range opts {
		o(clientData)
	}

	return map[any]any{constants.ClientData: clientData}
}

// WithAdditionalGroup provides an option for getting a ClientMap from an AuthClient.
// It adds an additional group to the ClientData Groups
func WithAdditionalGroup(groupName string) ClientMapOpt {
	return func(cd *auth.ClientData) {
		cd.Groups = append(cd.Groups, groupName)
	}
}

// WithOverriddenIdentifier provides an option for getting a ClientMap from an AuthClient.
// It overrides the AuthClient Identifier. This can be used, for example,
// when testing for other users in (or not in) the AuthClient Group
func WithOverriddenIdentifier(identifier string) ClientMapOpt {
	return func(cd *auth.ClientData) {
		cd.Identifier = identifier
	}
}

// WithOverriddenGroup provides an option for getting a ClientMap from an AuthClient.
// It overrides the AuthClient Groups. This can be used, for example,
// when testing for invalid groups for a given AuthClient identifier
func WithOverriddenGroup(numGroups int) ClientMapOpt {
	return func(cd *auth.ClientData) {
		cd.Groups = make([]string, numGroups)
		for i := range numGroups {
			cd.Groups[i] = uuid.NewString()
		}
	}
}

// WithAuthClientDataKC provides an option for the NewKeyConfig function
// This option will initialise the KeyConfig with the AuthClient Group
func WithAuthClientDataKC(authClient AuthClientData) KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *authClient.Group
		kc.AdminGroupID = authClient.Group.ID
	}
}

// AuthClientOpt are options which can be used with NewAuthClient
type AuthClientOpt func(*AuthClientData)

// GetAuthClientMap does the same as the NewAuthClient, except it returns the ClientMap directly.
// It can be used for simple tests when a separate AuthClient is not required
func GetAuthClientMap(ctx context.Context, tb testing.TB, r repo.Repo, opts ...AuthClientOpt) map[any]any {
	tb.Helper()
	authClient := newAuthClient(opts...)
	CreateTestEntities(ctx, tb, r, authClient.Group)
	return authClient.GetClientMap()
}

// WithAuditorRole provides an option for getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It specifies TenantAuditorRole for the group
func WithAuditorRole() AuthClientOpt {
	return func(acd *AuthClientData) {
		acd.Group.Role = constants.TenantAuditorRole
	}
}

// WithKeyAdminRole provides an option for getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It specifies KeyAdminRole for the group
func WithKeyAdminRole() AuthClientOpt {
	return func(acd *AuthClientData) {
		acd.Group.Role = constants.KeyAdminRole
	}
}

// WithTenantAdminRole provides an option for getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It specifies TenantAdminRole for the group
func WithTenantAdminRole() AuthClientOpt {
	return func(acd *AuthClientData) {
		acd.Group.Role = constants.TenantAdminRole
	}
}

// WithIdentifier provides an option when getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It allows the default random value for the AuthClient
// Identifier to be overridden with the provided value
func WithIdentifier(identifier string) AuthClientOpt {
	return func(acd *AuthClientData) {
		acd.Identifier = identifier
	}
}

// GetClientMap returns a client map created with the provided identifier and group names
// It does not create anything in the database
func GetClientMap(identifier string, groupNames []string) map[any]any {
	return map[any]any{constants.ClientData: getClientData(identifier, groupNames)}
}

// GetGrouplessClientMap returns a client map with a random identifier and no groupnames
// It does not create anything in the database
func GetGrouplessClientMap() map[any]any {
	return map[any]any{constants.ClientData: getClientData(uuid.NewString(), []string{})}
}

// GetInvalidClientMap returns a client map with random identifier and random groupnames
// It does not create anything in the database
func GetInvalidClientMap(opts ...ClientMapOpt) map[any]any {
	clientData := getClientData(uuid.NewString(), []string{uuid.NewString(), uuid.NewString()})
	return map[any]any{constants.ClientData: clientData}
}

func newAuthClient(opts ...AuthClientOpt) AuthClientData {
	group := NewGroup(func(g *model.Group) {
		g.ID = uuid.New()
		g.Name = uuid.NewString()
		g.IAMIdentifier = uuid.NewString()
		g.Role = constants.TenantAuditorRole
	})

	authClientData := AuthClientData{
		Group:      group,
		GroupID:    group.ID.String(),
		Identifier: uuid.NewString(),
	}

	for _, o := range opts {
		o(&authClientData)
	}

	return authClientData
}

func getClientData(identifier string, groupNames []string) *auth.ClientData {
	return &auth.ClientData{
		Identifier: identifier,
		Groups:     groupNames,
	}
}

// NewSignedClientDataHeaders generates HTTP headers with signed client data for testing
// This creates the x-client-data and x-client-data-signature headers that ClientDataMiddleware expects
// Uses RS256 algorithm (RSA + SHA-256) for signing
func NewSignedClientDataHeaders(tb testing.TB, clientData map[string]any, privateKey *rsa.PrivateKey, keyID int) http.Header {
	tb.Helper()

	// Convert map to auth.ClientData struct
	cd := &auth.ClientData{
		Identifier:         getString(clientData, "identifier"),
		Type:               getString(clientData, "type"),
		Email:              getString(clientData, "email"),
		Region:             getString(clientData, "region"),
		Groups:             getStringSlice(clientData, "groups"),
		KeyID:              strconv.Itoa(keyID),
		SignatureAlgorithm: auth.SignatureAlgorithmRS256,
		AuthContext:        getStringMap(clientData, "authContext"),
	}

	// Generate signed headers using the auth package
	clientDataHeader, signatureHeader, err := cd.Encode(privateKey)
	if err != nil {
		tb.Fatalf("Failed to encode and sign client data: %v", err)
	}

	// Create HTTP headers
	headers := http.Header{}
	headers.Set(auth.HeaderClientData, clientDataHeader)
	headers.Set(auth.HeaderClientDataSignature, signatureHeader)

	return headers
}

// NewSignedClientDataHeadersFromStruct generates HTTP headers from an auth.ClientData struct
// This is a convenience function for tests that already have ClientData objects
func NewSignedClientDataHeadersFromStruct(tb testing.TB, clientData *auth.ClientData, privateKey *rsa.PrivateKey, keyID int) http.Header {
	tb.Helper()

	// Set required fields for signing
	clientData.KeyID = strconv.Itoa(keyID)
	clientData.SignatureAlgorithm = auth.SignatureAlgorithmRS256

	// Generate signed headers using the auth package
	clientDataHeader, signatureHeader, err := clientData.Encode(privateKey)
	if err != nil {
		tb.Fatalf("Failed to encode and sign client data: %v", err)
	}

	// Create HTTP headers
	headers := http.Header{}
	headers.Set(auth.HeaderClientData, clientDataHeader)
	headers.Set(auth.HeaderClientDataSignature, signatureHeader)

	return headers
}

// Helper to get string from map, returns empty string if not found or wrong type
func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

// Helper to get string slice from map, returns empty slice if not found or wrong type
func getStringSlice(m map[string]any, key string) []string {
	if val, ok := m[key]; ok {
		if sliceVal, ok := val.([]string); ok {
			return sliceVal
		}
		// Handle []interface{} case
		if ifaceSlice, ok := val.([]any); ok {
			result := make([]string, 0, len(ifaceSlice))
			for _, item := range ifaceSlice {
				if strVal, ok := item.(string); ok {
					result = append(result, strVal)
				}
			}
			return result
		}
	}
	return []string{}
}

// Helper to get string map from map, returns empty map if not found or wrong type
func getStringMap(m map[string]any, key string) map[string]string {
	if val, ok := m[key]; ok {
		if mapVal, ok := val.(map[string]string); ok {
			return mapVal
		}
		// Handle map[string]interface{} case
		if ifaceMap, ok := val.(map[string]any); ok {
			result := make(map[string]string)
			for k, v := range ifaceMap {
				if strVal, ok := v.(string); ok {
					result[k] = strVal
				}
			}
			return result
		}
	}
	return map[string]string{}
}
