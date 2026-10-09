package cmk_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/multitenancy"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func startAPISystemGroups(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string, *testutils.TestSigningKeyStorage) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	keyStorage := testutils.NewTestSigningKeyStorage(t)

	r := testutils.NewAPIServer(
		t, db, testutils.TestAPIServerConfig{
			EnableBusinessUserDataMW: true,
			SigningKeyStorage:        keyStorage,
		},
	)

	return db, r, tenants[0], keyStorage
}

func TestGetAllSystemGroups(t *testing.T) {
	db, sv, tenant, keyStorage := startAPISystemGroups(t)
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())
	headers := testutils.WithBusinessUserData(t, keyStorage, authClient)

	g1 := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
	g2 := testutils.NewSystemGroup(func(g *model.SystemGroup) {})

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		g1,
		g2,
	)

	t.Run("Should code 200 and return the system groups", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.SystemGroupList](t, w)

		ids := make(map[uuid.UUID]struct{}, len(response.Value))
		for _, sg := range response.Value {
			ids[*sg.Id] = struct{}{}
		}
		assert.Contains(t, ids, g1.ID)
		assert.Contains(t, ids, g2.ID)
	})

	t.Run("Should code 500 on server failure", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Should include count on odata count", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups?$count=true",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.SystemGroupList](t, w)
		assert.NotNil(t, response.Count)
		assert.Equal(t, 2, *response.Count)
	})
}

func TestGetAllSystemGroupsExpand(t *testing.T) {
	db, sv, tenant, keyStorage := startAPISystemGroups(t)
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())
	headers := testutils.WithBusinessUserData(t, keyStorage, authClient)

	group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
	connected := testutils.NewSystem(func(s *model.System) {
		s.SystemGroupID = &group.ID
		s.Status = cmkapi.SystemStatusCONNECTED
	})
	disconnected := testutils.NewSystem(func(s *model.System) {
		s.SystemGroupID = &group.ID
		s.Status = cmkapi.SystemStatusDISCONNECTED
	})
	testutils.CreateTestEntities(ctx, t, r, group, connected, disconnected)

	t.Run("Should embed systems and compute warn coverage when expanded", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups?$expand=systems",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.SystemGroupList](t, w)
		for _, g := range response.Value {
			assert.NotEmpty(t, g.Systems)
		}
		assert.NotNil(t, response)
	})

	t.Run("Should not embed systems without expand", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.SystemGroupList](t, w)
		for _, g := range response.Value {
			assert.Empty(t, g.Systems)
		}
	})
}

func TestUpdateSystemGroup(t *testing.T) {
	db, sv, tenant, keyStorage := startAPISystemGroups(t)
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithKeyAdminRole())
	headers := testutils.WithBusinessUserData(t, keyStorage, authClient)

	t.Run("Should code 200 on successful update", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		patch := cmkapi.SystemGroupPatch{
			Name:            new("group-updated"),
			Description:     new("updated description"),
			SuppressWarning: new(true),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/systemGroups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, patch),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		response := testutils.GetJSONBody[cmkapi.SystemGroup](t, w)
		assert.Equal(t, "group-updated", response.Name)
		assert.NotNil(t, response.Description)
		assert.Equal(t, "updated description", *response.Description)
		assert.NotNil(t, response.SuppressWarning)
		assert.True(t, *response.SuppressWarning)
	})

	t.Run("Should code 400 on invalid id format", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: "/systemGroups/not-a-uuid",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.SystemGroupPatch{Name: new("x")}),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 400 on empty name", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/systemGroups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.SystemGroupPatch{Name: new("   ")}),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 404 on non-existing group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/systemGroups/%s", uuid.New()),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.SystemGroupPatch{Name: new("whatever")}),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should code 500 on server failure", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.WithUpdate().Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/systemGroups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.SystemGroupPatch{Name: new("new-name")}),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestSystemGroupAuthz(t *testing.T) {
	db, sv, tenant, keyStorage := startAPISystemGroups(t)
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should code 403 on get without system group read permission", func(t *testing.T) {
		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithTenantAdminRole())
		headers := testutils.WithBusinessUserData(t, keyStorage, authClient)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/systemGroups",
			Tenant:   tenant,
			Headers:  headers,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should code 403 on update with read-only role", func(t *testing.T) {
		authClient := testutils.NewAuthClient(ctx, t, r, testutils.WithAuditorRole())
		headers := testutils.WithBusinessUserData(t, keyStorage, authClient)

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/systemGroups/%s", uuid.New()),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, cmkapi.SystemGroupPatch{Name: new("x")}),
			Headers:  headers,
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
