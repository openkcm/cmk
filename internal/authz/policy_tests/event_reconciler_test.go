package authz_policy_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

// TestEventReconciler_AuthzPolicy verifies that the InternalEventReconcilerRole
// policy grants sufficient repo access for the CryptoReconciler
//
// A key and a CONNECTED system sharing the same KeyConfigurationID are seeded so
// that getRegionsByKeyID exercises Key:First and System:Count+List.
func TestEventReconciler_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalEventReconcilerRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	cfg := &config.Config{Database: dbCfg}

	reconciler, err := eventprocessor.NewCryptoReconciler(
		t.Context(), cfg, authzRepo, testutils.NewTestPlugins(), nil,
	)
	assert.NoError(t, err)

	keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		s.Status = cmkapi.SystemStatusCONNECTED
	})
	testutils.CreateTestEntities(ctx, t, r, keyConfig, key, system)

	t.Run("InternalEventReconcilerRole allows deriving connected regions for key", func(t *testing.T) {
		data := eventprocessor.KeyActionJobData{
			TenantID: tenant,
			KeyID:    key.ID.String(),
		}
		dataBytes, err := json.Marshal(data)
		assert.NoError(t, err)

		job := orbital.NewJob(eventprocessor.JobTypeKeyEnable.String(), dataBytes)
		handler, err := reconciler.GetHandlerByJobType(eventprocessor.JobTypeKeyEnable.String())
		assert.NoError(t, err)

		_, err = handler.ResolveTasks(ctx, job)
		// Check error is not authz related
		assert.ErrorIs(t, err, eventprocessor.ErrNoConnectedRegionsForKey)
	})

	t.Run("InternalEventReconcilerRole allows KeyVersion:First", func(t *testing.T) {
		kv := testutils.NewKeyVersion(func(k *model.KeyVersion) {
			k.KeyID = key.ID
		})
		require.NoError(t, r.Create(ctx, kv))

		var got model.KeyVersion
		ck := repo.NewCompositeKey().Where(
			fmt.Sprintf("%s_%s", repo.KeyField, repo.IDField), key.ID.String(),
		)
		query := repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)).
			Order(repo.OrderField{Field: repo.RotatedField, Direction: repo.Desc})
		_, err = authzRepo.First(ctx, &got, *query)
		assert.NoError(t, err)
	})

	t.Run("InternalEventReconcilerRole allows Event:Update and Event:Delete", func(t *testing.T) {
		identifier := "authz-test-event-identifier"
		event := &model.Event{
			Identifier: identifier,
			Type:       "test.event.type",
			Data:       []byte(`{}`),
			Status:     "FAILED",
		}
		require.NoError(t, r.Create(ctx, event))

		// Exercise Event:Update via Patch
		patchEvent := &model.Event{Identifier: identifier, ErrorCode: "TEST_CODE", ErrorMessage: "test error"}
		_, err = r.Patch(ctx, patchEvent, *repo.NewQuery())
		assert.NoError(t, err)

		// Exercise Event:Delete
		_, err = r.Delete(ctx, &model.Event{}, *repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(repo.NewCompositeKey().Where(repo.IdentifierField, identifier)),
		))
		assert.NoError(t, err)
	})
}
