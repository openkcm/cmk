package testutils

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

// AuthBusinessUserData contains a group and an identifier associated with an AuthClient
type AuthBusinessUserData struct {
	Group      *model.Group
	GroupID    string // For convenience. Just a string version of the Group.ID
	Identifier string
}

// ClientMapOpt are options which can be used, for example, when retrieving the
// BusinessUserData from an AuthClient
type ClientMapOpt func(*auth.ClientData)

// NewAuthClient creates an AuthClient using random strings for values and creates
// the group in the database
func NewAuthClient(ctx context.Context, tb testing.TB, r repo.Repo, opts ...AuthClientOpt) AuthBusinessUserData {
	tb.Helper()
	authClient := newAuthClient(opts...)
	CreateTestEntities(ctx, tb, r, authClient.Group)
	return authClient
}

// GetClientMap gets the ClientMap from the AuthClient. This can be used to authenticate
func (cd AuthBusinessUserData) GetClientMap(opts ...ClientMapOpt) map[any]any {
	businessUserData := getBusinessUserData(cd.Identifier, []string{cd.Group.IAMIdentifier})

	for _, o := range opts {
		o(businessUserData)
	}

	return map[any]any{constants.UserType: constants.BusinessUser,
		constants.BusinessUserData: businessUserData}
}

// WithBusinessUserData builds signed client-data headers for the given auth client.
func WithBusinessUserData(
	tb testing.TB,
	keyStorage *TestSigningKeyStorage,
	authClient AuthBusinessUserData,
	opts ...ClientMapOpt,
) http.Header {
	tb.Helper()

	clientData := getBusinessUserData(authClient.Identifier, []string{authClient.Group.IAMIdentifier})
	for _, o := range opts {
		o(clientData)
	}

	privateKey, ok := keyStorage.GetPrivateKey(0)
	if !ok {
		tb.Fatalf("test key should exist")
	}

	return NewSignedBusinessUserDataHeaders(tb, clientData, privateKey, 0)
}

// WithAdditionalGroup provides an option for getting a ClientMap from an AuthClient.
// It adds an additional group to the BusinessUserData Groups
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

// WithAuthBusinessUserDataKC provides an option for the NewKeyConfig function
// This option will initialise the KeyConfig with the AuthClient Group
func WithAuthBusinessUserDataKC(authClient AuthBusinessUserData) KeyConfigOpt {
	return func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *authClient.Group
		kc.AdminGroupID = authClient.Group.ID
	}
}

// AuthClientOpt are options which can be used with NewAuthClient
type AuthClientOpt func(*AuthBusinessUserData)

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
	return func(acd *AuthBusinessUserData) {
		acd.Group.Role = constants.TenantAuditorRole
	}
}

// WithKeyAdminRole provides an option for getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It specifies KeyAdminRole for the group
func WithKeyAdminRole() AuthClientOpt {
	return func(acd *AuthBusinessUserData) {
		acd.Group.Role = constants.KeyAdminRole
	}
}

// WithTenantAdminRole provides an option for getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It specifies TenantAdminRole for the group
func WithTenantAdminRole() AuthClientOpt {
	return func(acd *AuthBusinessUserData) {
		acd.Group.Role = constants.TenantAdminRole
	}
}

// WithIdentifier provides an option when getting an AuthClient with NewAuthClient, or the
// ClientMap with GetAuthClientMap. It allows the default random value for the AuthClient
// Identifier to be overridden with the provided value
func WithIdentifier(identifier string) AuthClientOpt {
	return func(acd *AuthBusinessUserData) {
		acd.Identifier = identifier
	}
}

// GetClientMap returns a client map created with the provided identifier and group names
// It does not create anything in the database
func GetClientMap(identifier string, groupNames []string) map[any]any {
	return map[any]any{constants.UserType: constants.BusinessUser,
		constants.BusinessUserData: getBusinessUserData(identifier, groupNames)}
}

// GetGrouplessClientMap returns a client map with a random identifier and no groupnames
// It does not create anything in the database
func GetGrouplessClientMap() map[any]any {
	return map[any]any{constants.UserType: constants.BusinessUser,
		constants.BusinessUserData: getBusinessUserData(uuid.NewString(), []string{})}
}

// GetInvalidClientMap returns a client map with random identifier and random groupnames
// It does not create anything in the database
func GetInvalidClientMap(opts ...ClientMapOpt) map[any]any {
	businessUserData := getBusinessUserData(uuid.NewString(), []string{uuid.NewString(), uuid.NewString()})
	return map[any]any{constants.UserType: constants.BusinessUser,
		constants.BusinessUserData: businessUserData}
}

func newAuthClient(opts ...AuthClientOpt) AuthBusinessUserData {
	group := NewGroup(func(g *model.Group) {
		g.ID = uuid.New()
		g.Name = uuid.NewString()
		g.IAMIdentifier = uuid.NewString()
		g.Role = constants.TenantAuditorRole
	})

	authBusinessUserData := AuthBusinessUserData{
		Group:      group,
		GroupID:    group.ID.String(),
		Identifier: uuid.NewString(),
	}

	for _, o := range opts {
		o(&authBusinessUserData)
	}

	return authBusinessUserData
}

func getBusinessUserData(identifier string, groupNames []string) *auth.ClientData {
	return &auth.ClientData{
		Identifier: identifier,
		Groups:     groupNames,
	}
}

// NewSignedBusinessUserDataHeaders generates HTTP headers from an auth.ClientData struct.
func NewSignedBusinessUserDataHeaders(
	tb testing.TB,
	clientData *auth.ClientData,
	privateKey *rsa.PrivateKey,
	keyID int,
) http.Header {
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

// AuthzTestEndpoint defines an API endpoint to test for authorization failures.
type AuthzTestEndpoint struct {
	// Method is the HTTP method (e.g., http.MethodGet)
	Method string
	// Endpoint is the URL path with any required path params filled in
	// (e.g., "/keys?keyConfigurationID=xxx")
	Endpoint string
	// Body is an optional JSON request body for POST/PATCH/PUT requests
	Body string
}

// WithBody converts a JSON string to an io.Reader for use as a request body.
// Returns nil if the body is empty.
func WithBody(tb testing.TB, body string) io.Reader {
	tb.Helper()

	if body == "" {
		return nil
	}

	return strings.NewReader(body)
}

// allRoles returns all defined roles.
func allRoles() []constants.BusinessRole {
	return []constants.BusinessRole{
		constants.KeyAdminRole,
		constants.TenantAdminRole,
		constants.TenantAuditorRole,
	}
}

// RoleAuthClientOpt maps a role to the corresponding AuthClientOpt.
func RoleAuthClientOpt(role constants.BusinessRole) AuthClientOpt {
	switch role {
	case constants.KeyAdminRole:
		return WithKeyAdminRole()
	case constants.TenantAdminRole:
		return WithTenantAdminRole()
	case constants.TenantAuditorRole:
		return WithAuditorRole()
	default:
		panic(fmt.Sprintf("unsupported role: %s", role))
	}
}

// GetAllowedRoles returns roles that have the given resource type + action
// based on the API policy data.
func GetAllowedRoles(
	resourceType authz.APIResourceTypeName,
	action authz.APIAction,
) []constants.BusinessRole {
	allowed := make(map[constants.BusinessRole]struct{})

	for _, policy := range authz.PolicyData.Policies {
		for _, rt := range policy.ResourceTypes {
			if rt.ID != resourceType {
				continue
			}

			if slices.Contains(rt.Actions, action) {
				allowed[policy.Role] = struct{}{}
			}
		}
	}

	var roles []constants.BusinessRole
	for _, role := range allRoles() {
		if _, ok := allowed[role]; ok {
			roles = append(roles, role)
		}
	}

	return roles
}

// GetBlockedRoles returns roles that do NOT have the given
// resource type + action.
func GetBlockedRoles(
	resourceType authz.APIResourceTypeName,
	action authz.APIAction,
) []constants.BusinessRole {
	allowed := GetAllowedRoles(resourceType, action)
	allowedSet := make(map[constants.BusinessRole]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}

	var blocked []constants.BusinessRole
	for _, role := range allRoles() {
		if _, ok := allowedSet[role]; !ok {
			blocked = append(blocked, role)
		}
	}

	return blocked
}

// CleanPath returns a sanitized version of the path for use in test names.
func CleanPath(path string) string {
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "?", "_")
	path = strings.ReplaceAll(path, "&", "_")
	path = strings.ReplaceAll(path, "=", "_")

	if len(path) > 0 && path[0] == '_' {
		path = path[1:]
	}

	return path
}

// SubstitutePathParams replaces path parameters (e.g., {keyID}) with dummy
// UUIDs so the path can be used in an HTTP request for route matching.
func SubstitutePathParams(path string) string {
	result := path
	for strings.Contains(result, "{") {
		start := strings.Index(result, "{")
		end := strings.Index(result, "}")

		if start == -1 || end == -1 {
			break
		}

		result = result[:start] + uuid.New().String() + result[end+1:]
	}

	return result
}
