package cmk_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func startAPIGroups(t *testing.T) (*multitenancy.DB, *http.ServeMux, string) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.Group{}, &model.KeyConfiguration{}},
	})

	r := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	return db, r, tenants[0]
}

func TestGetGroups(t *testing.T) {
	db, r, tenant := startAPIGroups(t)

	t.Run("Should code 200 on successful groups get", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/groups",
			Tenant:   tenant,
		})
		assert.Equal(t, http.StatusOK, w.Code)

		var response cmkapi.GroupList

		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
	})

	t.Run("Should code 500 on server failure", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/groups",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestPostGroups(t *testing.T) {
	_, r, tenant := startAPIGroups(t)

	t.Run("Should code 201 on successful group creation", func(t *testing.T) {
		group := cmkapi.Group{
			Name: "test",
			Role: cmkapi.GroupRoleKEYADMINISTRATOR,
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: "/groups",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, group),
		})

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Should code 400 on group with invalid role", func(t *testing.T) {
		group := cmkapi.Group{
			Name: "test",
			Role: cmkapi.GroupRoleTENANTAUDITOR,
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: "/groups",
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, group),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 400 on create group with invalid body", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPost,
			Endpoint: "/groups",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDeleteGroup(t *testing.T) {
	db, r, tenant := startAPIGroups(t)

	t.Run("Should code 204 on successful group delete", func(t *testing.T) {
		group := &model.Group{
			ID:   uuid.New(),
			Name: "test",
			Role: "test",
		}

		repo := sql.NewRepository(db)
		err := repo.Create(testutils.CreateCtxWithTenant(tenant), group)
		assert.NoError(t, err)

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodDelete,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Should code 404 on non-existing group delete", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodDelete,
			Endpoint: fmt.Sprintf("/groups/%s", uuid.New()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should code 400 on delete with invalid group id", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodDelete,
			Endpoint: "/groups/s",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 500 on server fail", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodDelete,
			Endpoint: fmt.Sprintf("/groups/%s", uuid.New()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestGetGroupID(t *testing.T) {
	db, r, tenant := startAPIGroups(t)

	t.Run("Should code 200 successful get", func(t *testing.T) {
		group := &model.Group{
			ID:   uuid.New(),
			Name: "test",
			Role: "test",
		}

		repo := sql.NewRepository(db)
		err := repo.Create(testutils.CreateCtxWithTenant(tenant), group)
		assert.NoError(t, err)

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should code 400 on wrong id format", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: "/groups/s",
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 404 on non existing group", func(t *testing.T) {
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/groups/%s", uuid.New()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should code 500 on server fail", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodGet,
			Endpoint: fmt.Sprintf("/groups/%s", uuid.New()),
			Tenant:   tenant,
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUpdateGroup(t *testing.T) {
	db, r, tenant := startAPIGroups(t)

	group := &model.Group{
		ID:   uuid.New(),
		Name: "test",
		Role: "test",
	}

	rep := sql.NewRepository(db)
	err := rep.Create(testutils.CreateCtxWithTenant(tenant), group)
	assert.NoError(t, err)

	t.Run("Should code 200 on successful group rename", func(t *testing.T) {
		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo("test"),
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateGroup),
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should code 400 on invalid group rename object", func(t *testing.T) {
		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo(""),
		}
		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateGroup),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 400 on rename to protect group name", func(t *testing.T) {
		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo(constants.TenantAdminGroup),
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateGroup),
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should code 404 on non existing group", func(t *testing.T) {
		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo("test"),
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/groups/%s", uuid.New()),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateGroup),
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should code 500 on server error", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		updateGroup := cmkapi.GroupPatch{
			Name: ptr.PointTo("test"),
		}

		w := testutils.MakeHTTPRequest(t, r, testutils.RequestOptions{
			Method:   http.MethodPatch,
			Endpoint: fmt.Sprintf("/groups/%s", group.ID),
			Tenant:   tenant,
			Body:     testutils.WithJSON(t, updateGroup),
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
