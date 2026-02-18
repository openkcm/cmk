package cmk_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	cmksql "github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var errMockInternalError = errors.New("internal error")

func startAPIWorkflows(t *testing.T) (*multitenancy.DB, cmkapi.ServeMux, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{
		Config: config.Config{Database: dbCfg},
	})

	return db, sv, tenants[0]
}

var (
	auditorGroupName  = "auditors"
	keyAdminGroupName = "keyAdmins"

	userID      = "008cfcb6-0a68-449e-bbf3-ef6ee8537f02"
	groupID     = "7a3834b8-1e41-4adc-bda2-73c72ad1d560"
	keyConfigID = "7a3834b8-1e41-4adc-bda2-73c72ad1d561"
	key1ID      = "7a3834b8-1e41-4adc-bda2-73c72ad1d562"
	key2ID      = "7a3834b8-1e41-4adc-bda2-73c72ad1d563"
	systemID    = "7a3834b8-1e41-4adc-bda2-73c72ad1d564"

	adminGroupIAMIdentifier = "admin-group-iam-identifier"
)

func createAuditorGroup(ctx context.Context, tb testing.TB, r repo.Repo) {
	tb.Helper()

	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = auditorGroupName
		g.IAMIdentifier = auditorGroupName
		g.Role = constants.TenantAuditorRole
	})
	testutils.CreateTestEntities(ctx, tb, r, group)
}

func createTestWorkflows(ctx context.Context, tb testing.TB, r repo.Repo) []*model.Workflow {
	tb.Helper()

	group := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = adminGroupIAMIdentifier
	})
	groupIDsBytes, err := json.Marshal([]uuid.UUID{group.ID})
	assert.NoError(tb, err)

	system := testutils.NewSystem(func(w *model.System) {})
	keyConfig := testutils.NewKeyConfig(func(w *model.KeyConfiguration) {
		w.AdminGroupID = group.ID
	})

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	key2 := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})

	workflow := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: userID}}
		w.ApproverGroupIDs = groupIDsBytes
		w.State = wfMechanism.StateWaitApproval.String()
		w.ArtifactType = wfMechanism.ArtifactTypeKey.String()
		w.ActionType = wfMechanism.ActionTypeDelete.String()
		w.ArtifactID = key.ID
		w.ArtifactName = &key.Name
	})

	workflow2 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.State = wfMechanism.StateRevoked.String()
		w.ActionType = wfMechanism.ActionTypeUpdateState.String()
		w.ArtifactType = wfMechanism.ArtifactTypeKey.String()
		w.ArtifactID = key2.ID
		w.ArtifactName = &key2.Name
		w.Approvers = []model.WorkflowApprover{{UserID: uuid.NewString()}}
		w.ApproverGroupIDs = groupIDsBytes
		w.Parameters = "DISABLED"
	})

	wfID := uuid.New()
	workflow3 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{
			{
				UserID:     userID,
				Approved:   sql.NullBool{Bool: true, Valid: true},
				WorkflowID: wfID,
			},
			{
				UserID:     uuid.NewString(),
				Approved:   sql.NullBool{Bool: false, Valid: true},
				WorkflowID: wfID,
			},
			{
				UserID:     uuid.NewString(),
				Approved:   sql.NullBool{Bool: false, Valid: false},
				WorkflowID: wfID,
			},
		}
		w.ID = wfID
		w.ApproverGroupIDs = groupIDsBytes
		w.State = wfMechanism.StateWaitApproval.String()
		w.ActionType = wfMechanism.ActionTypeLink.String()
		w.ArtifactType = wfMechanism.ArtifactTypeSystem.String()
		w.ArtifactID = system.ID
		w.ArtifactName = &system.Identifier
		w.Parameters = keyConfig.ID.String()
		w.ParametersResourceName = &keyConfig.Name
		w.ParametersResourceType = ptr.PointTo(wfMechanism.ParametersResourceTypeKeyConfiguration.String())
	})

	testutils.CreateTestEntities(ctx, tb, r, group, key, key2, system, keyConfig, workflow, workflow2, workflow3)

	return []*model.Workflow{workflow, workflow2, workflow3}
}

func setupTestWorkflowControllerCreateWorkflow(t *testing.T) (*multitenancy.DB,
	*cmksql.ResourceRepository, cmkapi.ServeMux, string, context.Context,
) {
	t.Helper()

	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createTestWorkflows(ctx, t, r)

	group := testutils.NewGroup(func(g *model.Group) {
		g.ID = uuid.MustParse(groupID)
		g.Name = keyAdminGroupName
		g.IAMIdentifier = keyAdminGroupName
		g.Role = constants.KeyAdminRole
	})

	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.MustParse(key1ID)
		k.KeyConfigurationID = uuid.MustParse(keyConfigID)
	})

	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.ID = uuid.MustParse(keyConfigID)
		c.AdminGroup = *group
		c.AdminGroupID = uuid.MustParse(groupID)
		c.PrimaryKeyID = &key.ID
	})

	key2 := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.MustParse(key2ID)
		k.KeyConfigurationID = uuid.MustParse(keyConfigID)
	})

	system := testutils.NewSystem(func(w *model.System) {
		w.ID = uuid.MustParse(systemID)
		w.KeyConfigurationID = ptr.PointTo(uuid.MustParse(keyConfigID))
	})

	testutils.CreateTestEntities(ctx, t, r, group, key, keyConfig, key2, system)

	// Do a create to ensure that the config is created. We need this for any
	// tests simulating a DB failure, otherwise the config creation will hit the
	// simulated error.
	wf := cmkapi.Workflow{
		ActionType:   cmkapi.WorkflowActionType(wfMechanism.ActionTypeUnlink),
		ArtifactID:   uuid.MustParse(systemID),
		ArtifactType: cmkapi.WorkflowArtifactType(wfMechanism.ArtifactTypeSystem),
	}

	clientData := map[any]any{
		constants.ClientData: &auth.ClientData{
			Identifier: uuid.NewString(),
			Groups:     []string{keyAdminGroupName},
		},
	}

	w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
		Method:            http.MethodPost,
		Endpoint:          "/workflows/check",
		Tenant:            tenant,
		Body:              testutils.WithJSON(t, wf),
		AdditionalContext: clientData,
	})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	return db, r, sv, tenant, ctx
}

func TestWorkflowControllerCheckWorkflow(t *testing.T) {
	t.Run("should 200 with valid and canCreate as true", func(t *testing.T) {
		_, _, sv, tenant, _ := setupTestWorkflowControllerCreateWorkflow(t)

		clientData := map[any]any{
			constants.ClientData: &auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{keyAdminGroupName},
			},
		}

		wf := cmkapi.Workflow{
			ActionType:   cmkapi.WorkflowActionType(wfMechanism.ActionTypeUnlink),
			ArtifactID:   uuid.MustParse(systemID),
			ArtifactType: cmkapi.WorkflowArtifactType(wfMechanism.ArtifactTypeSystem),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPost,
			Endpoint:          "/workflows/check",
			Tenant:            tenant,
			Body:              testutils.WithJSON(t, wf),
			AdditionalContext: clientData,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		res := testutils.GetJSONBody[cmkapi.CheckWorkflow200JSONResponse](t, w)
		assert.False(t, *res.Exists)
		assert.True(t, *res.Valid)
		assert.True(t, *res.CanCreate)
		assert.True(t, *res.Required)
		assert.Nil(t, res.Details)
	})

	t.Run("Should 200 with valid and canCreate as false on wf system connect invalid key state", func(t *testing.T) {
		_, r, sv, tenant, ctx := setupTestWorkflowControllerCreateWorkflow(t)

		groupName := uuid.NewString()
		group := testutils.NewGroup(func(g *model.Group) {
			g.Name = groupName
			g.IAMIdentifier = groupName
			g.Role = constants.KeyAdminRole
		})

		clientData := map[any]any{
			constants.ClientData: &auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{groupName},
			},
		}

		key := testutils.NewKey(func(k *model.Key) {
			k.State = string(cmkapi.KeyStateUNKNOWN)
		})

		keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
			c.AdminGroup = *group
			c.AdminGroupID = group.ID
			c.PrimaryKeyID = &key.ID
		})

		system := testutils.NewSystem(func(w *model.System) {
			w.KeyConfigurationID = &keyConfig.ID
		})

		testutils.CreateTestEntities(ctx, t, r, group, key, keyConfig, system)

		wf := cmkapi.Workflow{
			ActionType:   cmkapi.WorkflowActionTypeEnumLINK,
			ArtifactID:   system.ID,
			ArtifactType: cmkapi.WorkflowArtifactTypeEnumSYSTEM,
			Parameters:   ptr.PointTo(keyConfig.ID.String()),
		}

		w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
			Method:            http.MethodPost,
			Endpoint:          "/workflows/check",
			Tenant:            tenant,
			Body:              testutils.WithJSON(t, wf),
			AdditionalContext: clientData,
		})

		assert.Equal(t, http.StatusOK, w.Code)

		res := testutils.GetJSONBody[cmkapi.CheckWorkflow200JSONResponse](t, w)
		assert.False(t, *res.Exists)
		assert.False(t, *res.Valid)
		assert.False(t, *res.CanCreate)
		assert.True(t, *res.Required)
		assert.Equal(t, manager.ErrConnectSystemNoPrimaryKey.Error(), *res.Details)
	})
}

func TestWorkflowControllerCreateWorkflow(t *testing.T) {
	tests := []struct {
		name           string
		extraResource  []repo.Resource
		request        string
		sideEffect     func(db *multitenancy.DB) func()
		expectedStatus int
	}{
		{
			name: "TestWorkflowControllerCreateWorkflow_Okay_NoParams",
			request: `{
				"actionType":"UNLINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM"
			}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_Okay_WithParams",
			request: `{
				"actionType":"LINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"parameters": "` + keyConfigID + `"
			}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_WithExpires",
			request: `{
				"actionType":"UNLINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"expiresAt": "2002-10-02T10:00:00-05:00"
			}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_ValidationError_WithExpires",
			request: `{
				"actionType":"UNLINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"expiresAt": "xsxs"
			}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_OngoingWorkflow",
			extraResource: []repo.Resource{
				testutils.NewWorkflow(func(w *model.Workflow) {
					w.ArtifactID = uuid.MustParse(systemID)
					w.ArtifactType = wfMechanism.ArtifactTypeSystem.String()
					w.ActionType = wfMechanism.ActionTypeUnlink.String()
					w.State = wfMechanism.StateExecuting.String()
					w.Parameters = keyConfigID
				}),
			},
			request: `{
				"actionType":"LINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM"
				"parameters": "` + keyConfigID + `"
			}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_InternalError",
			request: `{
				"actionType":"UNLINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM"
			}`,
			sideEffect: func(db *multitenancy.DB) func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithCreate().Register()

				return errForced.Unregister
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "TestWorkflowControllerCreateWorkflow_InvalidBody",
			request:        "some-string",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerCreateWorkflow_NotJSON",
			request:        "{,,}",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, sv, tenant, ctx := setupTestWorkflowControllerCreateWorkflow(t)

			if tt.sideEffect != nil {
				teardown := tt.sideEffect(db)
				defer teardown()
			}

			testutils.CreateTestEntities(ctx, t, cmksql.NewRepository(db), tt.extraResource...)

			clientData := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: uuid.NewString(),
					Groups:     []string{keyAdminGroupName},
				},
			}
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          "/workflows",
				Tenant:            tenant,
				Body:              testutils.WithString(t, tt.request),
				AdditionalContext: clientData,
			})
			assert.Equal(t, tt.expectedStatus, w.Code, w.Body.String())
		})
	}
}

func TestWorkflowControllerCheckCreateWorkflowAuthz(t *testing.T) {
	keyAdminGroup2Name := "keyAdminGroup2"
	group2ID := uuid.MustParse("7a3834b8-1e41-4adc-bda3-73c72ad1d560")

	KAGroup := testutils.NewGroup(func(g *model.Group) {
		g.ID = group2ID
		g.Name = keyAdminGroup2Name
		g.IAMIdentifier = keyAdminGroup2Name
		g.Role = constants.KeyAdminRole
	})

	tenantAdminGroupName := "tenantAdminGroup"
	group3ID := uuid.MustParse("7a3834b8-1e41-4adc-bda4-73c72ad1d560")

	TAGroup := testutils.NewGroup(func(g *model.Group) {
		g.ID = group3ID
		g.Name = tenantAdminGroupName
		g.IAMIdentifier = tenantAdminGroupName
		g.Role = constants.TenantAdminRole
	})

	tenantAuditorGroupName := "tenantAuditorGroup"
	group4ID := uuid.MustParse("7a3834b8-1e41-4adc-bda5-73c72ad1d560")

	TAuditGroup := testutils.NewGroup(func(g *model.Group) {
		g.ID = group4ID
		g.Name = tenantAuditorGroupName
		g.IAMIdentifier = tenantAuditorGroupName
		g.Role = constants.TenantAuditorRole
	})

	tests := []struct {
		name                 string
		clientData           auth.ClientData
		expectedCheckStatus  int
		expectedCreateStatus int
	}{
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InKAGroup",
			clientData: auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{keyAdminGroupName},
			},
			expectedCheckStatus:  http.StatusOK,
			expectedCreateStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InOtherKAGroup",
			clientData: auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{keyAdminGroup2Name},
			},
			expectedCheckStatus:  http.StatusForbidden,
			expectedCreateStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InTAGroup",
			clientData: auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{tenantAdminGroupName},
			},
			expectedCheckStatus:  http.StatusForbidden,
			expectedCreateStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InTAuditGroup",
			clientData: auth.ClientData{
				Identifier: uuid.NewString(),
				Groups:     []string{tenantAuditorGroupName},
			},
			expectedCheckStatus:  http.StatusForbidden,
			expectedCreateStatus: http.StatusForbidden,
		},
	}

	requests := []string{
		`{
			"actionType":"UNLINK",
			"artifactID":"` + systemID + `",
			"artifactType":"SYSTEM"
		}`,
		`{
			"actionType":"LINK",
			"artifactID":"` + systemID + `",
			"artifactType":"SYSTEM",
			"parameters": "` + keyConfigID + `"
		}`,
		`{
			"actionType":"SWITCH",
			"artifactID":"` + systemID + `",
			"artifactType":"SYSTEM",
			"parameters": "` + keyConfigID + `"
		}`,
	}
	for _, tt := range tests {
		for _, request := range requests {
			t.Run(tt.name, func(t *testing.T) {
				_, r, sv, tenant, ctx := setupTestWorkflowControllerCreateWorkflow(t)

				testutils.CreateTestEntities(ctx, t, r, KAGroup, TAGroup, TAuditGroup)

				clientData := map[any]any{
					constants.ClientData: &tt.clientData,
				}

				w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/workflows/check",
					Tenant:            tenant,
					Body:              testutils.WithString(t, request),
					AdditionalContext: clientData,
				})
				assert.Equal(t, tt.expectedCheckStatus, w.Code, w.Body.String())

				w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
					Method:            http.MethodPost,
					Endpoint:          "/workflows",
					Tenant:            tenant,
					Body:              testutils.WithString(t, request),
					AdditionalContext: clientData,
				})
				assert.Equal(t, tt.expectedCreateStatus, w.Code, w.Body.String())
			})
		}
	}

	keyConfigWithoutUserID := "7a3834b8-1e41-4adc-cda2-73c72ad1d561"

	keyConfigWithoutUser := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.ID = uuid.MustParse(keyConfigWithoutUserID)
	})

	tests2 := []struct {
		name                 string
		request              string
		expectedCheckStatus  int
		expectedCreateStatus int
	}{
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InLinkSystem",
			request: `{
				"actionType":"LINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"parameters": "` + keyConfigID + `"}`,
			expectedCheckStatus:  http.StatusOK,
			expectedCreateStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_InSwitchSystem",
			request: `{
				"actionType":"SWITCH",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"parameters": "` + keyConfigID + `"}`,
			expectedCheckStatus:  http.StatusOK,
			expectedCreateStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_NotInLinkSystem",
			request: `{
				"actionType":"LINK",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"parameters": "` + keyConfigWithoutUserID + `"}`,
			expectedCheckStatus:  http.StatusForbidden,
			expectedCreateStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerCheckCreateWorkflowAuthz_NotInSwitchSystem",
			request: `{
				"actionType":"SWITCH",
				"artifactID":"` + systemID + `",
				"artifactType":"SYSTEM",
				"parameters": "` + keyConfigWithoutUserID + `"}`,
			expectedCheckStatus:  http.StatusForbidden,
			expectedCreateStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests2 {
		t.Run(tt.name, func(t *testing.T) {
			_, r, sv, tenant, ctx := setupTestWorkflowControllerCreateWorkflow(t)

			testutils.CreateTestEntities(ctx, t, r, keyConfigWithoutUser, KAGroup)

			clientData := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: uuid.NewString(),
					Groups:     []string{keyAdminGroupName},
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          "/workflows/check",
				Tenant:            tenant,
				Body:              testutils.WithString(t, tt.request),
				AdditionalContext: clientData,
			})
			assert.Equal(t, tt.expectedCheckStatus, w.Code, w.Body.String())

			w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodPost,
				Endpoint:          "/workflows",
				Tenant:            tenant,
				Body:              testutils.WithString(t, tt.request),
				AdditionalContext: clientData,
			})
			assert.Equal(t, tt.expectedCreateStatus, w.Code, w.Body.String())
		})
	}
}

func TestWorkflowControllerGetByID(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	workflows := createTestWorkflows(ctx, t, r)

	groupIDsBytes, err := json.Marshal([]uuid.UUID{uuid.New()})
	assert.NoError(t, err)

	workflowWithDeletedGroup := testutils.NewWorkflow(func(w *model.Workflow) {
		w.ActionType = wfMechanism.ActionTypeUpdateState.String()
		w.State = wfMechanism.StateWaitApproval.String()
		w.ArtifactType = wfMechanism.ArtifactTypeKey.String()
		w.ArtifactID = workflows[1].ArtifactID
		w.ArtifactName = workflows[1].ArtifactName
		w.Approvers = []model.WorkflowApprover{{UserID: userID}}
		w.ApproverGroupIDs = groupIDsBytes
		w.Parameters = "DISABLED"
	})
	testutils.CreateTestEntities(ctx, t, r, workflowWithDeletedGroup)

	tests := []struct {
		name              string
		workflowID        string
		sideEffect        func() func()
		userID            string
		expectedStatus    int
		approverGroupName string
	}{
		{
			name:           "TestWorkflowControllerGetByID_Okay_KeyDelete",
			workflowID:     workflows[0].ID.String(),
			userID:         workflows[0].InitiatorID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "TestWorkflowControllerGetByID_Okay_SystemLink",
			workflowID:     workflows[2].ID.String(),
			userID:         workflows[2].InitiatorID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "TestWorkflowControllerGetByID_InvalidUUID",
			workflowID:     "invalid-uuid",
			userID:         userID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerGetByID_NotFound",
			workflowID:     uuid.NewString(),
			userID:         userID,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:              "TestWorkflowControllerGetByID_DeletedGroup",
			workflowID:        workflowWithDeletedGroup.ID.String(),
			userID:            workflowWithDeletedGroup.InitiatorID,
			expectedStatus:    http.StatusOK,
			approverGroupName: "NOT_AVAILABLE",
		},
		{
			name: "TestWorkflowControllerGetByID_InternalError",
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithQuery().Register()

				return errForced.Unregister
			},
			workflowID:     workflows[0].ID.String(),
			userID:         userID,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			additionalContext := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: tt.userID,
					Groups:     []string{adminGroupIAMIdentifier},
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/workflows/" + tt.workflowID,
				Tenant:            tenant,
				AdditionalContext: additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.DetailedWorkflow](t, w)
				assert.Equal(t, tt.workflowID, response.Id.String())
				assert.Equal(t, tt.userID, response.InitiatorID)
				assert.NotNil(t, response.ArtifactName)
				assert.NotEmpty(t, response.AvailableTransitions)
				assert.NotNil(t, response.ApprovalSummary)
				if tt.approverGroupName != "" {
					assert.Equal(t, tt.approverGroupName, response.ApproverGroups[0].Name)
				}
			}
		})
	}
}

func TestWorkflowControllerListWorkflows(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	workflows := createTestWorkflows(ctx, t, r)
	createAuditorGroup(ctx, t, r)

	tests := []struct {
		name           string
		sideEffect     func() func()
		clientData     auth.ClientData
		expectedStatus int
		expectedCount  int
		count          bool
	}{
		{
			name: "TestWorkflowControllerListWorkflows_Okay_AsAuditor",
			clientData: auth.ClientData{
				Identifier: userID,
				Groups:     []string{auditorGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			count:          false,
		},
		{
			name: "TestWorkflowControllerListWorkflows_Okay_AsInitiator",
			clientData: auth.ClientData{
				Identifier: workflows[0].InitiatorID,
				Groups:     []string{keyAdminGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			count:          false,
		},
		{
			name: "TestWorkflowControllerListWorkflows_Okay_AsApprover",
			clientData: auth.ClientData{
				Identifier: userID,
				Groups:     []string{keyAdminGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			count:          false,
		},
		{
			name: "TestWorkflowControllerListWorkflowsWithCount_Okay_AsAuditor",
			clientData: auth.ClientData{
				Identifier: userID,
				Groups:     []string{auditorGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			count:          true,
		},
		{
			name: "TestWorkflowControllerListWorkflowsWithCount_Okay_AsInitiator",
			clientData: auth.ClientData{
				Identifier: workflows[2].InitiatorID,
				Groups:     []string{keyAdminGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			count:          true,
		},
		{
			name: "TestWorkflowControllerListWorkflowsWithCount_Okay_AsApprover",
			clientData: auth.ClientData{
				Identifier: userID,
				Groups:     []string{keyAdminGroupName},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			count:          true,
		},
		{
			name: "TestWorkflowControllerListWorkflows_InternalError",
			clientData: auth.ClientData{
				Identifier: userID,
				Groups:     []string{auditorGroupName},
			},
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithQuery().Register()

				return errForced.Unregister
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			path := "/workflows"
			if tt.count {
				path += "?$count=true"
			}

			additionalContext := map[any]any{
				constants.ClientData: &tt.clientData,
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          path,
				Tenant:            tenant,
				AdditionalContext: additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)

				if tt.count {
					assert.Equal(t, tt.expectedCount, *response.Count)
					assert.Len(t, response.Value, tt.expectedCount)
				} else {
					assert.Nil(t, response.Count)
					assert.Len(t, response.Value, tt.expectedCount)
				}
			}
		})
	}
}

func TestWorkflowControllerGetWorkflowsAuthz(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createAuditorGroup(ctx, t, r)

	user1ID := uuid.NewString()
	user2ID := uuid.NewString()

	workflow := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: user2ID}}
		w.InitiatorID = user1ID
	})

	workflow2 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: userID}}
		w.InitiatorID = user1ID
	})

	workflow3 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: user2ID}}
		w.InitiatorID = userID
	})

	workflow4 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: user2ID}}
		w.InitiatorID = userID
	})

	allWorkflows := []*model.Workflow{workflow, workflow2, workflow3, workflow4}

	testutils.CreateTestEntities(ctx, t, r, workflow, workflow2, workflow3, workflow4)

	tests := []struct {
		name             string
		groups           []string
		allowedWorkflows []*model.Workflow
	}{
		{
			name:             "user in auditor group",
			groups:           []string{auditorGroupName},
			allowedWorkflows: []*model.Workflow{workflow, workflow2, workflow3, workflow4},
		},
		{
			name:             "user not in auditor group",
			groups:           []string{},
			allowedWorkflows: []*model.Workflow{workflow2, workflow3, workflow4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			additionalContext := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: userID,
					Groups:     tt.groups,
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          "/workflows?$count=true",
				Tenant:            tenant,
				AdditionalContext: additionalContext,
			})

			assert.Equal(t, http.StatusOK, w.Code)
			response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)
			assert.Equal(t, len(tt.allowedWorkflows), *response.Count)

			for _, wf := range allWorkflows {
				w = testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
					Method:            http.MethodGet,
					Endpoint:          "/workflows/" + wf.ID.String(),
					Tenant:            tenant,
					AdditionalContext: additionalContext,
				})

				containsFunc := func(allowedWf *model.Workflow) bool {
					return allowedWf.ID == wf.ID
				}

				if slices.ContainsFunc(tt.allowedWorkflows, containsFunc) {
					assert.Equal(t, http.StatusOK, w.Code)
				} else {
					assert.Equal(t, http.StatusNotFound, w.Code)
				}
			}
		})
	}
}

func TestWorkflowControllerListWorkflowsWithPagination(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createAuditorGroup(ctx, t, r)

	for range totalRecordCount {
		workflow := testutils.NewWorkflow(func(_ *model.Workflow) {})
		testutils.CreateTestEntities(ctx, t, r, workflow)
	}

	tests := []struct {
		name               string
		query              string
		sideEffect         func() func()
		expectedStatus     int
		expectedSize       int
		expectedTotalCount int
		count              bool
	}{
		{
			name:               "GetWorkflowsDefaultPaginationValuesWithCount",
			query:              "/workflows?$count=true",
			expectedStatus:     http.StatusOK,
			expectedTotalCount: 21,
			expectedSize:       20,
			count:              true,
		},
		{
			name:           "GetWorkflowsDefaultPaginationValues",
			query:          "/workflows",
			expectedStatus: http.StatusOK,
			count:          false,
			expectedSize:   20,
		},
		{
			name:           "GetWorkflowsTopZero",
			query:          "/workflows?$top=0",
			count:          false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:               "GetWorkflowsOnlyTopParam",
			query:              "/workflows?$top=1",
			expectedStatus:     http.StatusOK,
			count:              false,
			expectedTotalCount: totalRecordCount,
			expectedSize:       1,
		},
		{
			name:           "GetWorkflows_Skip_0_Top_10",
			query:          "/workflows?$skip=0&$top=10",
			expectedStatus: http.StatusOK,
			count:          false,
			expectedSize:   10,
		},
		{
			name:               "GetWorkflows_Skip_0_Top_10_Count",
			query:              "/workflows?$skip=0&$top=10&$count=true",
			expectedStatus:     http.StatusOK,
			count:              true,
			expectedTotalCount: 21,
			expectedSize:       10,
		},
		{
			name:               "GetWorkflows_Skip_20_Top_10_Count",
			query:              "/workflows?$skip=20&$top=10&$count=true",
			expectedStatus:     http.StatusOK,
			count:              true,
			expectedTotalCount: 21,
			expectedSize:       1,
		},
		{
			name:           "GetWorkflows_Skip_20_Top_10",
			query:          "/workflows?$skip=20&$top=10",
			expectedStatus: http.StatusOK,
			count:          false,
			expectedSize:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			additionalContext := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: userID,
					Groups:     []string{auditorGroupName},
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          tt.query,
				Tenant:            tenant,
				AdditionalContext: additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)
				assert.Len(t, response.Value, tt.expectedSize)

				if tt.count {
					assert.Equal(t, tt.expectedTotalCount, *response.Count)
				} else {
					assert.Nil(t, response.Count)
				}
			}
		})
	}
}

func TestWorkflowControllerTransitionWorkflow(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createTestWorkflows(ctx, t, r)

	workflowID := uuid.New()
	initiatorID := uuid.NewString()
	approverID01 := uuid.NewString()
	approverID02 := uuid.NewString()

	wfMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:           workflowID,
			State:        wfMechanism.StateInitial.String(),
			InitiatorID:  initiatorID,
			ArtifactType: "KEY",
			ArtifactID:   uuid.New(),
			ActionType:   "DELETE",
			Approvers: []model.WorkflowApprover{
				{UserID: approverID01, Approved: repo.SQLNullBoolNull, WorkflowID: workflowID},
				{UserID: approverID02, Approved: repo.SQLNullBoolNull, WorkflowID: workflowID},
			},
		}
	})

	tests := []struct {
		name           string
		workflow       model.Workflow
		workflowID     string
		actorID        string
		request        string
		expectedStatus int
		expectedState  string
	}{
		{
			name:       "TestWorkflowControllerTransitionWorkflow_Approve_From_Initial",
			workflow:   wfMutator(),
			workflowID: workflowID.String(),
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Approve_As_Initiator",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Approve_As_First_Approver",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusOK,
			expectedState:  wfMechanism.StateWaitApproval.String(),
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Approve_As_Second_Approver",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.Approvers = []model.WorkflowApprover{
					{UserID: approverID01, Approved: sql.NullBool{Bool: true, Valid: true}, WorkflowID: workflowID},
					{UserID: approverID02, Approved: repo.SQLNullBoolNull, WorkflowID: workflowID},
				}
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        approverID02,
			expectedStatus: http.StatusOK,
			expectedState:  wfMechanism.StateWaitConfirmation.String(),
		},
		{
			name:       "TestWorkflowControllerTransitionWorkflow_Reject_From_Initial",
			workflow:   wfMutator(),
			workflowID: workflowID.String(),
			request: `{
				"transition": "REJECT"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Reject_As_Initiator",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "REJECT"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Revoke",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "REVOKE"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusOK,
			expectedState:  wfMechanism.StateRevoked.String(),
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Revoke_From_Revoked",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateRevoked.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "REVOKE"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Confirm",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitConfirmation.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "CONFIRM"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusOK,
			expectedState:  wfMechanism.StateFailed.String(),
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Confirm_As_Approver",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitConfirmation.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "CONFIRM"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "TestWorkflowControllerTransitionWorkflow_Confirm_From_Wait_Approval",
			workflow: wfMutator(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
			}),
			workflowID: workflowID.String(),
			request: `{
				"transition": "CONFIRM"
			}`,
			actorID:        initiatorID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerTransitionWorkflow_MalformedRequest",
			workflow:       wfMutator(),
			workflowID:     workflowID.String(),
			request:        `invalid-json`,
			actorID:        approverID01,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerTransitionWorkflow_InvalidUUID",
			workflow:   wfMutator(),
			workflowID: "invalid-uuid",
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerTransitionWorkflow_NotFound",
			workflow:   wfMutator(),
			workflowID: uuid.NewString(),
			request: `{
				"transition": "APPROVE"
			}`,
			actorID:        approverID01,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CreateTestEntities(ctx, t, r, &tt.workflow)

			defer func() {
				for _, approver := range tt.workflow.Approvers {
					testutils.DeleteTestEntities(ctx, t, r, &approver)
				}

				testutils.DeleteTestEntities(ctx, t, r, &tt.workflow)
			}()

			testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: fmt.Sprintf("/workflows/%s/state", tt.workflowID),
				Tenant:   tenant,
				Body:     testutils.WithString(t, tt.request),
				AdditionalContext: map[any]any{
					constants.ClientData: &auth.ClientData{
						Identifier: tt.actorID,
					},
				},
			})

			if tt.expectedState != "" {
				id, err := uuid.Parse(tt.workflowID)
				assert.NoError(t, err)

				workflow := &model.Workflow{ID: id}

				_, err = r.First(ctx, workflow, *repo.NewQuery())
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, workflow.State)
			}
		})
	}
}

func TestWorkflowControllerListWorkflows_WithFilters(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createAuditorGroup(ctx, t, r)
	workflows := createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "FilterByState_ValidState",
			query:          "/workflows?$filter=state eq 'REVOKED'",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByState_InvalidState",
			query:          "/workflows?$filter=state eq 'INVALID_STATE'",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByArtifactType_ValidType",
			query:          "/workflows?$filter=artifactType eq 'KEY'",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "FilterByArtifactType_InvalidType",
			query:          "/workflows?$filter=artifactType eq 'INVALID_TYPE'",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByActionType_ValidType",
			query:          "/workflows?$filter=actionType eq 'UPDATE_STATE'",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByActionType_ValidArtifactName",
			query:          fmt.Sprintf("/workflows?$filter=artifactName eq '%s'", *workflows[1].ArtifactName),
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name: "FilterByActionType_ValidParametersResourceName",
			query: fmt.Sprintf("/workflows?$filter=parametersResourceName eq '%s'",
				*workflows[2].ParametersResourceName),
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByActionType_InvalidType",
			query:          "/workflows?$filter=actionType eq 'INVALID_ACTION'",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByMultipleParameters",
			query:          "/workflows?$filter=state eq 'REVOKED' and artifactType eq 'KEY' and actionType eq 'UPDATE_STATE'",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			additionalContext := map[any]any{
				constants.ClientData: &auth.ClientData{
					Identifier: userID,
					Groups:     []string{auditorGroupName},
				},
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:            http.MethodGet,
				Endpoint:          tt.query,
				Tenant:            tenant,
				AdditionalContext: additionalContext,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)

				assert.Len(t, response.Value, tt.expectedCount)
			}
		})
	}
}
