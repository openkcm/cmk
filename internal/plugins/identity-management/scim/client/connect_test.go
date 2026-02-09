package client_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/magodo/slog2hclog"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
)

const (
	GetUserResponse = `{"id":"d1a6888d-7fd5-4c3f-ae33-177b24aae627",` +
		`"meta":{"created":"2020-04-10T11:29:36Z","lastModified":"2021-05-18T15:18:00Z",` +
		`"location":"https://a2e15w1y0.accounts400.ondemand.com/scim/Users/d1a6888d-7fd5-4c3f-ae33-177b24aae627",` +
		`"resourceType":"User", "groups.cnt":0}, "schemas":["urn:ietf:params:scim:schemas:core:2.0:User",` +
		`"urn:ietf:params:scim:schemas:extension:sap:2.0:User"], "userName":"cloudanalyst",` +
		`"name":{"familyName":"Analyst", "givenName":"Cloud"}, "displayName":"None", "userType":"employee",` +
		`"active":true, "emails":[{"value":"cloud.analyst@example.com", "primary":true}],` +
		`"groups":[{"value":"d1a6888d-7fd5-4c3f-ae33-177b24aae627", "display":"CloudAnalyst"}],` +
		`"urn:ietf:params:scim:schemas:extension:sap:2.0:User":` +
		`{"emails":[{"verified":false, "value":"cloud.analyst@example.com", "primary":true}],` +
		`"sourceSystem":0, "userUuid":"d1a6888d-7fd5-4c3f-ae33-177b24aae627",` +
		`"mailVerified":false, "userId":"P000011", "status":"active",` +
		`"passwordDetails":{"failedLoginAttempts":0, "setTime":"2020-04-10T11:29:36Z",` +
		`"status":"initial", "policy":"https://accounts.sap.com/policy/passwords/sap/web/1.1"}}}`
	ListUsersResponse = `{"Resources":[` + GetUserResponse + `],` +
		`"totalResults":1, "startIndex": 1, "itemsPerPage":1,` +
		`"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"]}`

	GetGroupResponse = `{"id":"16e720aa-a009-4949-9bf9-847fb0660522",` +
		`"meta":{"created":"2020-11-12T14:55:12Z","lastModified":"2021-03-31T14:56:01Z",` +
		`"location":"https://a2e15w1y0.accounts400.ondemand.com/scim/Groups/16e720aa-a009-4949-9bf9-847fb0660522",` +
		`"version":"f5c7bafe-b86f-4741-a35a-b53fe07b25e6","resourceType":"Group"},` +
		`"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group",` +
		`"urn:sap:cloud:scim:schemas:extension:custom:2.0:Group"],"displayName":"KeyAdmin",` +
		`"members":[{"value":"700223c4-3b58-4358-8594-59d14e619f4a","type":"User"}],` +
		`"urn:sap:cloud:scim:schemas:extension:custom:2.0:Group":{"name":"KeyAdmin",` +
		`"additionalId":"5f079f17cbf5f51d531d28f7","description":""}}`
	ListGroupsResponse = `{"Resources":[` + GetGroupResponse + `],` +
		`"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],` +
		`"totalResults":36,"itemsPerPage":100,"startIndex":1}`
)

var (
	ExpectedUser = client.User{
		BaseResource: client.BaseResource{
			ID:         "d1a6888d-7fd5-4c3f-ae33-177b24aae627",
			ExternalID: "",
			Meta:       struct{}{},
			Schemas: []string{
				"urn:ietf:params:scim:schemas:core:2.0:User",
				"urn:ietf:params:scim:schemas:extension:sap:2.0:User",
			},
		},
		UserName:    "cloudanalyst",
		Name:        struct{}{},
		DisplayName: "None",
		Active:      true,
		Emails: []client.MultiValuedAttribute{
			{
				Primary: true,
				Display: "",
				Value:   "cloud.analyst@example.com",
			},
		},
		Groups: []client.MultiValuedAttribute{
			{
				Display: "CloudAnalyst",
				Value:   "d1a6888d-7fd5-4c3f-ae33-177b24aae627",
			},
		},
		UserType: "employee",
	}
	ExpectedGroup = client.Group{
		BaseResource: client.BaseResource{
			ID:         "16e720aa-a009-4949-9bf9-847fb0660522",
			ExternalID: "",
			Meta:       struct{}{},
			Schemas: []string{
				"urn:ietf:params:scim:schemas:core:2.0:Group",
				"urn:sap:cloud:scim:schemas:extension:custom:2.0:Group",
			},
		},
		DisplayName: "KeyAdmin",
		Members: []client.MultiValuedAttribute{
			{
				Value: "700223c4-3b58-4358-8594-59d14e619f4a",
			},
		},
	}
)

func getLogger() hclog.Logger {
	logLevelPlugin := new(slog.LevelVar)
	logLevelPlugin.Set(slog.LevelError)

	return slog2hclog.New(slog.Default(), logLevelPlugin)
}

func getServer(t *testing.T, responseStatus int, responseBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(responseStatus)
		_, err := w.Write([]byte(responseBody))
		assert.NoError(t, err)
	}))
}

func getBasicClient() *client.Client {
	client, _ := client.NewClient(
		commoncfg.SecretRef{
			Type: commoncfg.BasicSecretType,
			Basic: commoncfg.BasicAuth{
				Username: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  ""},
				Password: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  ""},
			},
		}, getLogger())

	return client
}

func TestNewClient(t *testing.T) {
	exHost := "http://example.com"

	tests := []struct {
		name          string
		host          string
		auth          commoncfg.SecretRef
		expectError   bool
		errorContains string
	}{
		{
			name: "Non-supported auth",
			host: exHost,
			auth: commoncfg.SecretRef{
				Type: commoncfg.OAuth2SecretType,
			},
			expectError:   true,
			errorContains: "API Auth not implemented",
		},
		{
			name: "Basic auth",
			host: exHost,
			auth: commoncfg.SecretRef{
				Type: commoncfg.BasicSecretType,
				Basic: commoncfg.BasicAuth{
					Username: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  ""},
					Password: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  ""},
				},
			},
			expectError: false,
		},
		{
			name: "MTLS auth with bad cert",
			host: exHost,
			auth: commoncfg.SecretRef{
				Type: commoncfg.MTLSSecretType,
				MTLS: commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "bad"},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "bad"},
					ServerCA: &commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "bad"},
				},
			},
			expectError:   true,
			errorContains: "failed to parse client certificate x509 pair",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := client.NewClient(tt.auth, getLogger())

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		responseStatus int
		responseBody   string
		expectedUser   *client.User
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Success",
			userID:         "123",
			responseStatus: http.StatusOK,
			responseBody:   GetUserResponse,
			expectedUser:   &ExpectedUser,
			expectError:    false,
		},
		{
			name:           "User Not Found",
			userID:         "123",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"detail": "User not found"}`,
			expectedUser:   nil,
			expectError:    true,
			errorContains:  "error getting SCIM user",
		},
		{
			name:           "Invalid JSON",
			userID:         "123",
			responseStatus: http.StatusOK,
			responseBody:   `invalid-json`,
			expectedUser:   nil,
			expectError:    true,
			errorContains:  "error getting SCIM user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := getServer(t, tt.responseStatus, tt.responseBody)
			defer server.Close()

			c := getBasicClient()

			user, err := c.GetUser(t.Context(), tt.userID, client.RequestParams{Host: server.URL})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUser, user)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		responseStatus int
		responseBody   string
		expectedUsers  *client.UserList
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Success GET",
			responseStatus: http.StatusOK,
			responseBody:   ListUsersResponse,
			expectedUsers:  &client.UserList{Resources: []client.User{ExpectedUser}},
			expectError:    false,
		},
		{
			name:           "Success POST",
			method:         http.MethodPost,
			responseStatus: http.StatusOK,
			responseBody:   ListUsersResponse,
			expectedUsers:  &client.UserList{Resources: []client.User{ExpectedUser}},
			expectError:    false,
		},
		{
			name:           "Invalid JSON",
			responseStatus: http.StatusOK,
			responseBody:   `invalid-json`,
			expectedUsers:  nil,
			expectError:    true,
			errorContains:  "error listing SCIM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := getServer(t, tt.responseStatus, tt.responseBody)
			defer server.Close()

			c := getBasicClient()

			filter := client.FilterComparison{
				Attribute: "DisplayName",
				Operator:  client.FilterOperatorEqual,
				Value:     "None",
			}
			users, err := c.ListUsers(t.Context(), client.RequestParams{
				Host:   server.URL,
				Method: tt.method,
				Filter: filter,
			})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, users)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUsers, users)
			}
		})
	}
}

func TestGetGroup(t *testing.T) {
	tests := []struct {
		name           string
		groupID        string
		responseStatus int
		responseBody   string
		expectedGroup  *client.Group
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Success",
			groupID:        "123",
			responseStatus: http.StatusOK,
			responseBody:   GetGroupResponse,
			expectedGroup:  &ExpectedGroup,
			expectError:    false,
		},
		{
			name:           "Group Not Found",
			groupID:        "123",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"detail": "Group not found"}`,
			expectedGroup:  nil,
			expectError:    true,
			errorContains:  "error getting SCIM group",
		},
		{
			name:           "Invalid JSON",
			groupID:        "123",
			responseStatus: http.StatusOK,
			responseBody:   `invalid-json`,
			expectedGroup:  nil,
			expectError:    true,
			errorContains:  "error getting SCIM group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := getServer(t, tt.responseStatus, tt.responseBody)
			defer server.Close()

			c := getBasicClient()

			group, err := c.GetGroup(t.Context(), tt.groupID, "members", client.RequestParams{Host: server.URL})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, group)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedGroup, group)
			}
		})
	}
}

func TestListGroups(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		responseStatus int
		responseBody   string
		expectedGroups *client.GroupList
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Success GET",
			responseStatus: http.StatusOK,
			responseBody:   ListGroupsResponse,
			expectedGroups: &client.GroupList{Resources: []client.Group{ExpectedGroup}},
			expectError:    false,
		},
		{
			name:           "Success POST",
			method:         http.MethodPost,
			responseStatus: http.StatusOK,
			responseBody:   ListGroupsResponse,
			expectedGroups: &client.GroupList{Resources: []client.Group{ExpectedGroup}},
			expectError:    false,
		},
		{
			name:           "Invalid JSON",
			responseStatus: http.StatusOK,
			responseBody:   `invalid-json`,
			expectedGroups: nil,
			expectError:    true,
			errorContains:  "error listing SCIM groups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := getServer(t, tt.responseStatus, tt.responseBody)
			defer server.Close()

			c := getBasicClient()

			filter := client.FilterComparison{
				Attribute: "DisplayName",
				Operator:  client.FilterOperatorEqual,
				Value:     "KeyAdmin",
			}
			groups, err := c.ListGroups(t.Context(), client.RequestParams{
				Host:   server.URL,
				Method: tt.method,
				Filter: filter,
			})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, groups)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedGroups, groups)
			}
		})
	}
}
