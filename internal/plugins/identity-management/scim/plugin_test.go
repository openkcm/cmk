package scim_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/pointers"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	idmangv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/config"
)

const (
	NonExistentField = "Non-existent"
	GetUserResponse  = `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",` +
		`"meta":{"created":"2020-04-10T11:29:36Z","lastModified":"2021-05-18T15:18:00Z",` +
		`"location":"https://dummy.domain.com/scim/Users/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",` +
		`"resourceType":"User", "groups.cnt":0}, "schemas":["urn:ietf:params:scim:schemas:core:2.0:User",` +
		`"urn:ietf:params:scim:schemas:extension:comp:2.0:User"], "userName":"cloudanalyst",` +
		`"name":{"familyName":"Analyst", "givenName":"Cloud"}, "displayName":"None", "userType":"employee",` +
		`"active":true, "emails":[{"value":"cloud.analyst@example.com", "primary":true}],` +
		`"groups":[{"value":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "display":"CloudAnalyst"}],` +
		`"urn:ietf:params:scim:schemas:extension:comp:2.0:User":` +
		`{"emails":[{"verified":false, "value":"cloud.analyst@example.com", "primary":true}],` +
		`"sourceSystem":0, "userUuid":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",` +
		`"mailVerified":false, "userId":"P000011", "status":"active",` +
		`"passwordDetails":{"failedLoginAttempts":0, "setTime":"2020-04-10T11:29:36Z",` +
		`"status":"initial", "policy":"https://dummy.domain.com/policy/passwords/comp/web/1.1"}}}`
	ListUsersResponse = `{"Resources":[` + GetUserResponse + `],` +
		`"totalResults":1, "startIndex": 1, "itemsPerPage":1,` +
		`"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"]}`

	GetGroupResponse = `{"id":"16e720aa-a009-4949-9bf9-aaaaaaaaaaaa",` +
		`"meta":{"created":"2020-11-12T14:55:12Z","lastModified":"2021-03-31T14:56:01Z",` +
		`"location":"https://dummy.domain.com.com/scim/Groups/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",` +
		`"version":"f5c7bafe-b86f-4741-a35a-b53fe07b25e6","resourceType":"Group"},` +
		`"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group",` +
		`"urn:comp:cloud:scim:schemas:extension:custom:2.0:Group"],"displayName":"KeyAdmin",` +
		`"members":[{"value":"11111111-bbbb-cccc-dddd-ffffffffffff","type":"User"}],` +
		`"urn:comp:cloud:scim:schemas:extension:custom:2.0:Group":{"name":"KeyAdmin",` +
		`"additionalId":"5f079f17cbf5f51daaaaaaaa","description":""}}`
	ListGroupsResponse = `{"Resources":[` + GetGroupResponse + `],` +
		`"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],` +
		`"totalResults":1,"itemsPerPage":1,"startIndex":1}`
	EmptyResponse = `{"Resources":[],` +
		`"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],` +
		`"totalResults":0,"itemsPerPage":1,"startIndex":0}`
)

var NonExistentFieldPtr *string = pointers.To(NonExistentField)

func setupTest(t *testing.T, url string, groupFilterAttribute, userFilterAttribute string) *scim.Plugin {
	t.Helper()

	p := scim.NewPlugin()
	p.SetTestClient(t, url, groupFilterAttribute, userFilterAttribute)
	assert.NotNil(t, p)

	return p
}

func TestNoScimClient(t *testing.T) {
	p := scim.NewPlugin()

	groupRequest := idmangv1.GetUsersForGroupRequest{}
	_, err := p.GetUsersForGroup(t.Context(), &groupRequest)

	assert.Error(t, err)
	assert.ErrorIs(t, err, scim.ErrNoScimClient)

	userRequest := idmangv1.GetGroupsForUserRequest{}
	_, err = p.GetGroupsForUser(t.Context(), &userRequest)

	assert.Error(t, err)
	assert.ErrorIs(t, err, scim.ErrNoScimClient)
}

func TestGetAllGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(ListGroupsResponse))

		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name              string
		serverUrl         string
		testNumGroups     int
		testGroupId       string
		testGroupName     string
		testExpectedError *error
	}{
		{
			name:              "Bad Server",
			serverUrl:         "badurl",
			testNumGroups:     0,
			testGroupId:       "",
			testGroupName:     "",
			testExpectedError: &client.ErrListGroups,
		},
		{
			name:              "Good request",
			serverUrl:         server.URL,
			testNumGroups:     1,
			testGroupId:       "16e720aa-a009-4949-9bf9-aaaaaaaaaaaa",
			testGroupName:     "KeyAdmin",
			testExpectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupTest(t, tt.serverUrl, "", "")

			responseMsg, err := p.GetAllGroups(t.Context(),
				&idmangv1.GetAllGroupsRequest{})

			if tt.testExpectedError == nil {
				assert.NoError(t, err)
				assert.Len(t, responseMsg.GetGroups(), tt.testNumGroups)

				if tt.testNumGroups > 0 {
					assert.Equal(
						t,
						&idmangv1.GetAllGroupsResponse{
							Groups: []*idmangv1.Group{{
								Id:   tt.testGroupId,
								Name: tt.testGroupName}},
						},
						responseMsg,
					)
				}
			} else {
				assert.ErrorIs(t, err, *tt.testExpectedError)
			}
		})
	}
}

func TestGetUsersForGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		// Quick and dirty mock server filtering. Fine since we aren't testing server here
		reqStr := string(bodyBytes)
		if strings.Contains(reqStr, NonExistentField) {
			_, err = w.Write([]byte(EmptyResponse))
		} else {
			_, err = w.Write([]byte(ListUsersResponse))
		}

		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name                 string
		serverUrl            string
		groupFilterAttribute string
		groupFilterValue     string
		testNumUsers         int
		testUserEmail        string
		testUserName         string
		testUserID           string
		testExpectedError    *error
	}{
		{
			name:                 "Bad Server",
			serverUrl:            "badurl",
			groupFilterAttribute: "displayName",
			groupFilterValue:     "None",
			testNumUsers:         0,
			testUserEmail:        "",
			testUserName:         "",
			testUserID:           "",
			testExpectedError:    &client.ErrListUsers,
		},
		{
			name:                 "Good request",
			serverUrl:            server.URL,
			groupFilterAttribute: "displayName",
			groupFilterValue:     "None",
			testNumUsers:         1,
			testUserEmail:        "cloud.analyst@example.com",
			testUserName:         "cloudanalyst",
			testUserID:           "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			testExpectedError:    nil,
		},
		{
			name:                 "Non-existent filter value",
			serverUrl:            server.URL,
			groupFilterAttribute: "displayName",
			groupFilterValue:     "",
			testNumUsers:         0,
			testUserEmail:        "",
			testUserName:         "",
			testUserID:           "",
			testExpectedError:    &scim.ErrNoID,
		},
		{
			name:                 "Non-existent filter attribute",
			serverUrl:            server.URL,
			groupFilterAttribute: NonExistentField,
			groupFilterValue:     "None",
			testNumUsers:         0,
			testUserEmail:        "",
			testUserName:         "",
			testUserID:           "",
			testExpectedError:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupTest(t, tt.serverUrl, tt.groupFilterAttribute, "")

			var request = idmangv1.GetUsersForGroupRequest{}
			if tt.groupFilterValue != "" {
				request.GroupId = tt.groupFilterValue
			}

			responseMsg, err := p.GetUsersForGroup(t.Context(), &request)
			if tt.testExpectedError == nil {
				assert.NoError(t, err)
				assert.Len(t, responseMsg.GetUsers(), tt.testNumUsers)

				if tt.testNumUsers > 0 {
					assert.Equal(
						t,
						&idmangv1.GetUsersForGroupResponse{
							Users: []*idmangv1.User{{
								Id:    tt.testUserID,
								Name:  tt.testUserName,
								Email: tt.testUserEmail},
							},
						},
						responseMsg,
					)
				}
			} else {
				assert.ErrorIs(t, err, *tt.testExpectedError)
			}
		})
	}
}

func TestGetGroupsForUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		// Quick and dirty mock server filtering. Fine since we aren't testing server here
		reqStr := string(bodyBytes)
		if strings.Contains(reqStr, NonExistentField) {
			_, err = w.Write([]byte(EmptyResponse))
		} else {
			_, err = w.Write([]byte(ListGroupsResponse))
		}

		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name                string
		serverUrl           string
		userFilterAttribute string
		userFilterValue     string
		testNumGroups       int
		testGroupId         string
		testGroupName       string
		testExpectedError   *error
	}{
		{
			name:                "Bad Server",
			serverUrl:           "badurl",
			userFilterAttribute: "displayName",
			userFilterValue:     "None",
			testNumGroups:       0,
			testGroupId:         "",
			testGroupName:       "",
			testExpectedError:   &client.ErrListGroups,
		},
		{
			name:                "Good request",
			serverUrl:           server.URL,
			userFilterAttribute: "displayName",
			userFilterValue:     "None",
			testNumGroups:       1,
			testGroupId:         "16e720aa-a009-4949-9bf9-aaaaaaaaaaaa",
			testGroupName:       "KeyAdmin",
			testExpectedError:   nil,
		},
		{
			name:                "Non-existent filter value",
			serverUrl:           server.URL,
			userFilterAttribute: "displayName",
			userFilterValue:     NonExistentField,
			testNumGroups:       0,
			testGroupId:         "",
			testGroupName:       "",
			testExpectedError:   nil,
		},
		{
			name:                "Non-existent filter attribute",
			serverUrl:           server.URL,
			userFilterAttribute: NonExistentField,
			userFilterValue:     "None",
			testNumGroups:       0,
			testGroupId:         "",
			testGroupName:       "",
			testExpectedError:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupTest(t, tt.serverUrl, "", tt.userFilterAttribute)

			var userFilterValue = idmangv1.GetGroupsForUserRequest{}
			if tt.userFilterValue != "" {
				userFilterValue.UserId = tt.userFilterValue
			}

			responseMsg, err := p.GetGroupsForUser(t.Context(),
				&userFilterValue)

			if tt.testExpectedError == nil {
				assert.NoError(t, err)
				assert.Len(t, responseMsg.GetGroups(), tt.testNumGroups)

				if tt.testNumGroups > 0 {
					assert.Equal(
						t,
						&idmangv1.GetGroupsForUserResponse{
							Groups: []*idmangv1.Group{{
								Id:   tt.testGroupId,
								Name: tt.testGroupName}},
						},
						responseMsg,
					)
				}
			} else {
				assert.ErrorIs(t, err, *tt.testExpectedError)
			}
		})
	}
}

func TestNewPlugin(t *testing.T) {
	p := setupTest(t, "", "", "")
	assert.NotNil(t, p)
}

//nolint:cyclop
func TestCreateParams(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		authContextYAML := `
hostField: host
basePath: /api
headerFields:
  Authorization: Bearer token
`
		cfg := &config.Config{
			Host:        embeddedSourceRef("https://example.com"),
			AuthContext: embeddedSourceRef(authContextYAML),
			Params: config.Params{
				GroupAttribute:          embeddedSourceRef("groups"),
				UserAttribute:           embeddedSourceRef("users"),
				GroupMembersAttribute:   embeddedSourceRef("members"),
				ListMethod:              embeddedSourceRef("GET"),
				AllowSearchUsersByGroup: embeddedSourceRef("true"),
			},
		}

		params, err := scim.CreateParams(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if params.BaseHost != "https://example.com" {
			t.Errorf("BaseHost mismatch: %s", params.BaseHost)
		}
		if !params.AllowSearchUsersByGroup {
			t.Errorf("AllowSearchUsersByGroup expected true")
		}
		if params.AuthContext.BasePath != "/api" {
			t.Errorf("AuthContext not parsed correctly")
		}
	})

	t.Run("invalid boolean", func(t *testing.T) {
		cfg := &config.Config{
			Host:        embeddedSourceRef("https://example.com"),
			AuthContext: embeddedSourceRef(`hostField: host`),
			Params: config.Params{
				GroupAttribute:          embeddedSourceRef("groups"),
				UserAttribute:           embeddedSourceRef("users"),
				GroupMembersAttribute:   embeddedSourceRef("members"),
				ListMethod:              embeddedSourceRef("GET"),
				AllowSearchUsersByGroup: embeddedSourceRef("not-a-bool"),
			},
		}

		params, err := scim.CreateParams(cfg)
		if err == nil || params != nil {
			t.Fatal("expected error due to invalid boolean")
		}
	})

	t.Run("invalid auth context yaml", func(t *testing.T) {
		cfg := &config.Config{
			Host: embeddedSourceRef("https://example.com"),
			Params: config.Params{
				GroupAttribute:          embeddedSourceRef("groups"),
				UserAttribute:           embeddedSourceRef("users"),
				GroupMembersAttribute:   embeddedSourceRef("members"),
				ListMethod:              embeddedSourceRef("GET"),
				AllowSearchUsersByGroup: embeddedSourceRef("true"),
			},
		}

		params, err := scim.CreateParams(cfg)
		if err == nil || params != nil {
			t.Fatal("expected error due to invalid auth context yaml")
		}
	})

	t.Run("failed loading host", func(t *testing.T) {
		cfg := &config.Config{
			Host: commoncfg.SourceRef{}, // empty SourceRef → load failure
		}

		params, err := scim.CreateParams(cfg)
		if err == nil || params != nil {
			t.Fatal("expected error due to missing host")
		}
	})
}

func TestPlugin_Configure(t *testing.T) {
	cfg := config.Config{
		Host: embeddedSourceRef("example.com"),
	}
	yamlStr, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		yaml        string
		expectError bool
	}{
		{
			name:        "no credential found",
			yaml:        string(yamlStr),
			expectError: true,
		},
		{
			name:        "invalid yaml",
			yaml:        ":::",
			expectError: true,
		},
		{
			name:        "createParams fails",
			yaml:        "host: example.com",
			expectError: true,
		},
		{
			name:        "client creation fails",
			yaml:        "host: example.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &scim.Plugin{}

			req := &configv1.ConfigureRequest{
				YamlConfiguration: tt.yaml,
			}

			resp, err := p.Configure(context.Background(), req)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatal("expected response, got nil")
			}
		})
	}
}

func embeddedSourceRef(value string) commoncfg.SourceRef {
	return commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue,
		Value:  value,
	}
}
