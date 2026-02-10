package cmk_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func startAPIGroups(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	cfg := testutils.TestAPIServerConfig{
		Plugins: []testutils.MockPlugin{testutils.IdentityPlugin},
	}

	sv := testutils.NewAPIServer(
		t, db, cfg, &dbCfg,
	)

	return db, sv, tenants[0]
}

func TestGetGroups(t *testing.T) {
	db, r, tenant := startAPIGroups(t)
	repo := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	testutils.CreateTenantAdminGroup(ctx, t, repo)

	t.Run("Should code 200 on successful groups get", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/groups",
				Tenant:            tenant,
				AdditionalContext: testutils.GetTenantAdminClientMap(),
			},
		)

		assert.Equal(t, http.StatusOK, w.Code)

		var response cmkapi.GroupList

		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Value, 1)
	})

	t.Run("Should code 403 on empty groups when no client data", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/groups",
				Tenant:   tenant,
			},
		)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should code 500 on server failure", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/groups",
				Tenant:            tenant,
				AdditionalContext: testutils.GetTenantAdminClientMap(),
			},
		)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestPostGroups(t *testing.T) {
	db, r, tenant := startAPIGroups(t)
	rep := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	testutils.CreateTenantAdminGroup(ctx, t, rep)

	t.Run(
		"Should code 201 on successful group creation", func(t *testing.T) {
			group := cmkapi.Group{
				Name: "test",
				Role: cmkapi.GroupRoleKEYADMINISTRATOR,
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups",
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, group),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusCreated, w.Code)
		},
	)

	t.Run(
		"Should code 400 on group with a non applicable role", func(t *testing.T) {
			group := cmkapi.Group{
				Name: "test",
				Role: cmkapi.GroupRoleTENANTAUDITOR,
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups",
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, group),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 400 on group with an invalid role", func(t *testing.T) {
			group := cmkapi.Group{
				Name: "test",
				Role: "invalid role",
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups",
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, group),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 400 on group with invalid name", func(t *testing.T) {
			group := cmkapi.Group{
				Name: "$",
				Role: cmkapi.GroupRoleKEYADMINISTRATOR,
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups",
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, group),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 400 on create group with invalid body", func(t *testing.T) {
			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups",
					Tenant:            tenant,
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)
}

func TestDeleteGroup(t *testing.T) {
	db, r, tenant := startAPIGroups(t)
	repo := sql.NewRepository(db)
	testutils.CreateTenantAdminGroup(testutils.CreateCtxWithTenant(tenant), t, repo)

	t.Run(
		"Should code 204 on successful group delete", func(t *testing.T) {
			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodDelete,
					Endpoint:          "/groups/" + testutils.TenantAdminGroupID,
					Tenant:            tenant,
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusNoContent, w.Code)
		},
	)

	t.Run(
		"Should code 400 on delete with invalid group id", func(t *testing.T) {
			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:   http.MethodDelete,
					Endpoint: "/groups/s",
					Tenant:   tenant,
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 404 on non-existing group delete", func(t *testing.T) {
			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodDelete,
					Endpoint:          fmt.Sprintf("/groups/%s", uuid.New()),
					Tenant:            tenant,
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusNotFound, w.Code)
		},
	)

	t.Run(
		"Should code 500 on server fail", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)

			forced.Register()
			defer forced.Unregister()

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodDelete,
					Endpoint:          fmt.Sprintf("/groups/%s", uuid.New()),
					Tenant:            tenant,
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

}

func TestGetGroupID(t *testing.T) {
	db, r, tenant := startAPIGroups(t)
	rep := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)
	auditorGroup := testutils.CreateAuditorGroup(ctx, t, rep)

	t.Run("Should code 200 successful get", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/groups/%s", auditorGroup.ID),
				Tenant:            tenant,
				AdditionalContext: testutils.GetAuditorClientMap(),
			},
		)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should code 400 on wrong id format", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/groups/s",
				Tenant:   tenant,
			},
		)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 404 on non existing group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/groups/%s", uuid.New()),
				Tenant:            tenant,
				AdditionalContext: testutils.GetAuditorClientMap(),
			},
		)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should code 500 on server failure", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          fmt.Sprintf("/groups/%s", uuid.New()),
				Tenant:            tenant,
				AdditionalContext: testutils.GetAuditorClientMap(),
			},
		)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

}

func TestUpdateGroup(t *testing.T) {
	db, r, tenant := startAPIGroups(t)
	repo := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	TAGroup := testutils.CreateTenantAdminGroup(ctx, t, repo)
	otherTAGroup := testutils.CreateOtherTenantAdminGroup(ctx, t, repo, "otherTenantAdminGroup", nil)
	testutils.CreateKeyAdminGroup(ctx, t, repo)

	t.Run("Should code 200 on successful group rename", func(t *testing.T) {
		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo("test"),
		}

		w := testutils.MakeHTTPRequest(
			t, r, testutils.RequestOptions{
				Method:            http.MethodPatch,
				Endpoint:          "/groups/" + testutils.KeyAdminGroupID,
				Tenant:            tenant,
				Body:              testutils.WithJSON(t, updateGroup),
				AdditionalContext: testutils.GetTenantAdminClientMap(),
			},
		)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(
		"Should code 400 on invalid group rename object", func(t *testing.T) {
			updateGroup := cmkapi.GroupPatch{
				Name: ptr.PointTo(""),
			}
			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:   http.MethodPatch,
					Endpoint: fmt.Sprintf("/groups/%s", TAGroup.ID),
					Tenant:   tenant,
					Body:     testutils.WithJSON(t, updateGroup),
					AdditionalContext: testutils.GetClientGroupMap(testutils.KeyAdminName,
						otherTAGroup.Name),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 400 on rename to protect group name", func(t *testing.T) {
			updateGroup := cmkapi.GroupPatch{
				Name: ptr.PointTo(constants.TenantAdminGroup),
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:   http.MethodPatch,
					Endpoint: fmt.Sprintf("/groups/%s", otherTAGroup.ID),
					Tenant:   tenant,
					Body:     testutils.WithJSON(t, updateGroup),
					AdditionalContext: testutils.GetClientGroupMap(testutils.KeyAdminName,
						otherTAGroup.Name),
				},
			)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		},
	)

	t.Run(
		"Should code 404 on non existing group", func(t *testing.T) {
			updateGroup := cmkapi.GroupPatch{
				Name: ptr.PointTo("test"),
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPatch,
					Endpoint:          fmt.Sprintf("/groups/%s", uuid.New()),
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, updateGroup),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusNotFound, w.Code)
		},
	)

	t.Run(
		"Should code 500 on server failure", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)

			forced.Register()
			defer forced.Unregister()

			updateGroup := cmkapi.GroupPatch{
				Name: ptr.PointTo("test"),
			}

			w := testutils.MakeHTTPRequest(
				t, r, testutils.RequestOptions{
					Method:            http.MethodPatch,
					Endpoint:          fmt.Sprintf("/groups/%s", TAGroup.ID),
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, updateGroup),
					AdditionalContext: testutils.GetTenantAdminClientMap(),
				},
			)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
}

func TestCheckGroupsIAM(t *testing.T) {
	db, sv, tenant := startAPIGroups(t)
	r := sql.NewRepository(db)

	ctx := testutils.CreateCtxWithTenant(tenant)

	testutils.CreateAuditorGroup(ctx, t, r)

	t.Run(
		"returns correct response on success", func(t *testing.T) {
			body := cmkapi.CheckGroupsIAMJSONRequestBody{
				IamIdentifiers: []string{"KMS_001", "KMS_002", "KMS_999"},
			}
			w := testutils.MakeHTTPRequest(
				t, sv, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/groups/iamCheck",
					Tenant:            tenant,
					Body:              testutils.WithJSON(t, body),
					AdditionalContext: testutils.GetAuditorClientMap(),
				},
			)
			assert.Equal(t, http.StatusOK, w.Code)

			response := testutils.GetJSONBody[cmkapi.CheckGroupsIAM200JSONResponse](t, w)

			expected := cmkapi.CheckGroupsIAM200JSONResponse{
				Value: []cmkapi.GroupIAMExistence{
					{
						IamIdentifier: ptr.PointTo("KMS_001"),
						Exists:        true,
					},
					{
						IamIdentifier: ptr.PointTo("KMS_002"),
						Exists:        true,
					},
					{
						IamIdentifier: ptr.PointTo("KMS_999"),
						Exists:        false,
					},
				},
			}
			assert.Equal(t, expected, response)
		},
	)
}
