package authz_repo_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func TestAuthzScenarios(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})
	r := sql.NewRepository(db)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenants[0])

	cfg := config.Config{}
	authzRepoLoader := testutils.NewRepoAuthzLoader(ctx, r, &cfg)

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	adminGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "admin-group"
		g.Role = testutils.TestAdminRole
	})

	readGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "read-group"
		g.Role = testutils.TestReadAllowedRole
	})

	writeGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "write-group"
		g.Role = testutils.TestWriteAllowedRole
	})

	blockGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = "block-group"
		g.Role = testutils.TestBlockedRole
	})

	testutils.CreateTestEntities(ctx, t, r, adminGroup, readGroup, writeGroup, blockGroup)

	ctx = cmkcontext.InjectRequestID(ctx, "")

	tests := []struct {
		name           string
		group          string
		expectedList   error
		expectedFirst  error
		expectedCount  error
		expectedCreate error
		expectedPatch  error
		expectedSet    error
	}{
		{
			name:           "admin",
			group:          "admin-group",
			expectedList:   nil,
			expectedFirst:  nil,
			expectedCount:  nil,
			expectedCreate: nil,
			expectedPatch:  nil,
			expectedSet:    nil,
		},
		{
			name:           "write",
			group:          "write-group",
			expectedList:   authz.ErrAuthorizationDenied,
			expectedFirst:  authz.ErrAuthorizationDenied,
			expectedCount:  authz.ErrAuthorizationDenied,
			expectedCreate: nil,
			expectedPatch:  nil,
			expectedSet:    nil,
		},
		{
			name:           "read",
			group:          "read-group",
			expectedList:   nil,
			expectedFirst:  nil,
			expectedCount:  nil,
			expectedCreate: authz.ErrAuthorizationDenied,
			expectedPatch:  authz.ErrAuthorizationDenied,
			expectedSet:    authz.ErrAuthorizationDenied,
		},
		{
			name:           "block",
			group:          "block-group",
			expectedList:   authz.ErrAuthorizationDenied,
			expectedFirst:  authz.ErrAuthorizationDenied,
			expectedCount:  authz.ErrAuthorizationDenied,
			expectedCreate: authz.ErrAuthorizationDenied,
			expectedPatch:  authz.ErrAuthorizationDenied,
			expectedSet:    authz.ErrAuthorizationDenied,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutils.InjectClientDataIntoContext(
				ctx, "testuser", []string{tt.group})

			err := authzRepo.Create(ctx, &testutils.TestModel{
				ID: uuid.New(), Name: "test" + tt.group})
			assert.ErrorIs(t, err, tt.expectedCreate)

			ms := []*testutils.TestModel{}
			err = authzRepo.List(ctx, testutils.TestModel{}, &ms, *repo.NewQuery())
			assert.ErrorIs(t, err, tt.expectedList)

			m := &testutils.TestModel{}
			_, err = authzRepo.First(ctx, m, *repo.NewQuery())
			assert.ErrorIs(t, err, tt.expectedFirst)

			_, err = authzRepo.Count(ctx, testutils.TestModel{}, *repo.NewQuery())
			assert.ErrorIs(t, err, tt.expectedCount)
		})
	}
}
