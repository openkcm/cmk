package cmk_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	cmksql "github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	wfMechanism "github.com/openkcm/cmk-core/internal/workflow"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

var errMockInternalError = errors.New("internal error")

func startAPIWorkflows(t *testing.T) (*multitenancy.DB, *http.ServeMux, string) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Workflow{},
			&model.WorkflowApprover{},
			&model.Key{},
		},
	})

	sv := testutils.NewAPIServer(t, db, testutils.TestAPIServerConfig{})

	return db, sv, tenants[0]
}

func createTestWorkflows(ctx context.Context, tb testing.TB, r repo.Repo) []*model.Workflow {
	tb.Helper()

	userID, _ := uuid.Parse("008cfcb6-0a68-449e-bbf3-ef6ee8537f02")
	approverID, _ := uuid.Parse("76e06743-80c6-4372-a195-269e4473036d")

	workflow := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = []model.WorkflowApprover{{UserID: userID}}
	})

	workflow2 := testutils.NewWorkflow(func(w *model.Workflow) {
		w.State = wfMechanism.StateRevoked.String()
		w.ActionType = wfMechanism.ActionTypeUpdateState.String()
		w.Approvers = []model.WorkflowApprover{{UserID: approverID}}
		w.Parameters = "DISABLED"
	})
	testutils.CreateTestEntities(ctx, tb, r, workflow, workflow2)

	return []*model.Workflow{workflow, workflow2}
}

func TestWorkflowControllerCreateWorkflow(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createTestWorkflows(ctx, t, r)

	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.MustParse("7a3834b8-1e41-4adc-bda2-73c72ad1d564")
	})
	testutils.CreateTestEntities(ctx, t, r, key)

	tests := []struct {
		name           string
		request        string
		headers        map[string]string
		sideEffect     func() func()
		expectedStatus int
	}{
		{
			name: "TestWorkflowControllerCreateWorkflow_NoHeader",
			request: `{
				"actionType":"DELETE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d564",
				"artifactType":"KEY"
			}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_WrongHeader",
			request: `{
				"actionType":"DELETE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d564",
				"artifactType":"KEY"
			}`,
			headers:        map[string]string{"X-User-ID": uuid.NewString()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_Okay_NoParams",
			request: `{
				"actionType":"DELETE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d564",
				"artifactType":"KEY"
			}`,
			headers:        map[string]string{"User-ID": uuid.NewString()},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_Okay_WithParams",
			request: `{
				"actionType":"UPDATE_STATE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d565",
				"artifactType":"KEY",
                "parameters": "DISABLED"
			}`,
			headers:        map[string]string{"User-ID": uuid.NewString()},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_OngoingWorkflow",
			request: `{
				"actionType":"UPDATE_STATE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d565",
				"artifactType":"KEY",
                "parameters": "ENABLED"
			}`,
			headers:        map[string]string{"User-ID": uuid.NewString()},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "TestWorkflowControllerCreateWorkflow_InternalError",
			request: `{
				"actionType":"UPDATE_STATE",
				"artifactID":"7a3834b8-1e41-4adc-bda2-73c72ad1d566",
				"artifactType":"KEY",
                "parameters": "DISABLED"
			}`,
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithCreate().Register()

				return errForced.Unregister
			},
			headers:        map[string]string{"User-ID": uuid.NewString()},
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
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: "/workflows",
				Tenant:   tenant,
				Body:     testutils.WithString(t, tt.request),
				Headers:  tt.headers,
			})

			assert.Equal(t, tt.expectedStatus, w.Code, w.Body.String())

			if tt.expectedStatus == http.StatusOK {
				testutils.GetJSONBody[cmkapi.Workflow](t, w)
			}
		})
	}
}

func TestWorkflowControllerGetByID(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	workflows := createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		workflowID     string
		sideEffect     func() func()
		expectedStatus int
	}{
		{
			name:           "TestWorkflowControllerGetByID_Okay",
			workflowID:     workflows[0].ID.String(),
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "TestWorkflowControllerGetByID_InvalidUUID",
			workflowID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerGetByID_NotFound",
			workflowID:     uuid.NewString(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "TestWorkflowControllerGetByID_InternalError",
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithQuery().Register()

				return errForced.Unregister
			},
			workflowID:     workflows[0].ID.String(),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: "/workflows/" + tt.workflowID,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.Workflow](t, w)
				assert.Equal(t, tt.workflowID, response.Id.String())
			}
		})
	}
}

func TestWorkflowControllerListWorkflows(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		sideEffect     func() func()
		expectedStatus int
		expectedCount  int
		count          bool
	}{
		{
			name:           "TestWorkflowControllerListWorkflows_Okay",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			count:          false,
		},
		{
			name:           "TestWorkflowControllerListWorkflowsWithCount_Okay",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			count:          true,
		},
		{
			name: "TestWorkflowControllerListWorkflows_InternalError",
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

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: path,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)

				if tt.count {
					assert.Equal(t, tt.expectedCount, *response.Count)
				} else {
					assert.Nil(t, response.Count)
				}
			}
		})
	}
}

func TestWorkflowApproversPagination(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)

	approvers := make([]model.WorkflowApprover, 0, totalRecordCount)

	for range totalRecordCount {
		wa := testutils.NewWorkflowApprover(func(_ *model.WorkflowApprover) {})
		approvers = append(approvers, *wa)
	}

	workflow := testutils.NewWorkflow(func(w *model.Workflow) {
		w.Approvers = approvers
	})
	testutils.CreateTestEntities(ctx, t, r, workflow)

	tests := []struct {
		name               string
		query              string
		sideEffect         func() func()
		expectedStatus     int
		expectedErrorCode  string
		expectedSize       int
		expectedTotalCount int
		count              bool
	}{
		{
			name:           "GetWorkflowApproversDefaultPaginationValues",
			expectedStatus: http.StatusOK,
			query:          "/workflows/" + workflow.ID.String() + "/approvers",
			count:          false,
			expectedSize:   20,
		},
		{
			name:               "GetWorkflowApproversDefaultPaginationValuesWithCount",
			expectedStatus:     http.StatusOK,
			query:              "/workflows/" + workflow.ID.String() + "/approvers?$count=true",
			count:              true,
			expectedSize:       20,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GetWorkflowApproversTopZero",
			query:          "/workflows/" + workflow.ID.String() + "/approvers?$top=0",
			count:          false,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GetWorkflowApproversPaginationOnlyTopParam",
			query:          "/workflows/" + workflow.ID.String() + "/approvers?$top=3",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedSize:   3,
		},
		{
			name:               "GetWorkflowApproversPaginationOnlyTopParamWithCount",
			query:              "/workflows/" + workflow.ID.String() + "/approvers?$top=3&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       3,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GetWorkflowApproversPaginationTopAndSkipParams",
			query:              "/workflows/" + workflow.ID.String() + "/approvers?$skip=0&$top=10",
			count:              false,
			expectedStatus:     http.StatusOK,
			expectedSize:       10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:               "GetWorkflowApproversPaginationTopAndSkipParamsWithCount",
			query:              "/workflows/" + workflow.ID.String() + "/approvers?$skip=0&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       10,
			expectedTotalCount: totalRecordCount,
		},
		{
			name:           "GetWorkflowApproversPaginationTopAndSkipParamsLast",
			query:          "/workflows/" + workflow.ID.String() + "/approvers?$skip=20&$top=10",
			count:          false,
			expectedStatus: http.StatusOK,
			expectedSize:   1,
		},
		{
			name:               "GetWorkflowApproversPaginationTopAndSkipParamsLastWithCount",
			query:              "/workflows/" + workflow.ID.String() + "/approvers?$skip=20&$top=10&$count=true",
			count:              true,
			expectedStatus:     http.StatusOK,
			expectedSize:       1,
			expectedTotalCount: totalRecordCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowApproverList](t, w)

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

func TestWorkflowControllerListWorkflowsWithPagination(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)

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

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
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

func TestWorkflowControllerListWorkflowApproversByWorkflowID(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	workflows := createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		workflowID     string
		sideEffect     func() func()
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "TestWorkflowControllerListWorkflowApproversByWorkflowID_Okay",
			workflowID:     workflows[0].ID.String(),
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "TestWorkflowControllerListWorkflowApproversByWorkflowID_InvalidUUID",
			workflowID:     "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerListWorkflowApproversByWorkflowID_NotFound",
			workflowID:     uuid.NewString(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "TestWorkflowControllerListWorkflowApproversByWorkflowID_InternalError",
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithQuery().Register()

				return errForced.Unregister
			},
			workflowID:     workflows[0].ID.String(),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: fmt.Sprintf("/workflows/%s/approvers", tt.workflowID),
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowApproverList](t, w)
				assert.Len(t, response.Value, tt.expectedCount)
			}
		})
	}
}

func TestWorkflowControllerAddWorkflowApproversByWorkflowID(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	workflows := createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		workflowID     string
		request        string
		headers        map[string]string
		sideEffect     func() func()
		expectedStatus int
		expectedState  string
	}{
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_NotEnoughApprovers",
			workflowID: workflows[0].ID.String(),
			request: `{
				"approvers": []
			}`,
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_Conflict",
			workflowID: workflows[0].ID.String(),
			request: `{
				"approvers": [{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}, 
								{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusConflict,
			expectedState:  wfMechanism.StateInitial.String(),
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_InvalidWorkflowUUID",
			workflowID: "invalid-uuid",
			request: `{
				"approvers": [{"id": "4c36785f-3696-4c47-98e9-21f77ceb961f", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_NotFound",
			workflowID: uuid.NewString(),
			request: `{
				"approvers": [{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_InvalidApproverUUID",
			workflowID: "invalid-uuid",
			request: `{
				"approvers": [{"id": "new-approver-id", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_WrongActor",
			workflowID: workflows[0].ID.String(),
			request: `{
				"approvers": [{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[1].InitiatorID.String()},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_InternalErrorOnAdd",
			workflowID: workflows[0].ID.String(),
			request: `{
				"approvers": [{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}]
			}`,
			headers: map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.WithCreate().Register()

				return errForced.Unregister
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_InvalidState",
			workflowID: workflows[1].ID.String(),
			request: `{
				"approvers": [{"id": "178464b7-4433-4322-ad53-6be3fff4277d", "decision":"APPROVED"}]
			}`,
			headers:        map[string]string{"User-ID": workflows[1].InitiatorID.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerAddWorkflowApproversByWorkflowID_InternalErrorOnTransition",
			workflowID: workflows[0].ID.String(),
			request: `{
				"approvers": [{"id": "4c36785f-3696-4c47-98e9-21f77ceb961f", "decision":"APPROVED"}]
			}`,
			sideEffect: func() func() {
				errForced := testutils.NewDBErrorForced(db, errMockInternalError)
				errForced.Register()

				return errForced.Unregister
			},
			headers:        map[string]string{"User-ID": workflows[0].InitiatorID.String()},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sideEffect != nil {
				teardown := tt.sideEffect()
				defer teardown()
			}

			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodPost,
				Endpoint: fmt.Sprintf("/workflows/%s/approvers", tt.workflowID),
				Tenant:   tenant,
				Body:     testutils.WithString(t, tt.request),
				Headers:  tt.headers,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				testutils.GetJSONBody[cmkapi.WorkflowApproverList](t, w)
			}

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

func TestWorkflowControllerTransitionWorkflow(t *testing.T) {
	db, sv, tenant := startAPIWorkflows(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	r := cmksql.NewRepository(db)
	createTestWorkflows(ctx, t, r)

	workflowID := uuid.New()
	initiatorID := uuid.New()
	approverID01 := uuid.New()
	approverID02 := uuid.New()

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
		request        string
		headers        map[string]string
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
			headers:        map[string]string{"User-ID": approverID01.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
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
			headers:        map[string]string{"User-ID": approverID01.String()},
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
			headers:        map[string]string{"User-ID": approverID02.String()},
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
			headers:        map[string]string{"User-ID": approverID01.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
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
			headers:        map[string]string{"User-ID": approverID01.String()},
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
			headers:        map[string]string{"User-ID": initiatorID.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "TestWorkflowControllerTransitionWorkflow_MalformedRequest",
			workflow:       wfMutator(),
			workflowID:     workflowID.String(),
			request:        `invalid-json`,
			headers:        map[string]string{"User-ID": approverID01.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerTransitionWorkflow_InvalidUUID",
			workflow:   wfMutator(),
			workflowID: "invalid-uuid",
			request: `{
				"transition": "APPROVE"
			}`,
			headers:        map[string]string{"User-ID": approverID01.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:       "TestWorkflowControllerTransitionWorkflow_NotFound",
			workflow:   wfMutator(),
			workflowID: uuid.NewString(),
			request: `{
				"transition": "APPROVE"
			}`,
			headers:        map[string]string{"User-ID": approverID01.String()},
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
				Headers:  tt.headers,
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
	createTestWorkflows(ctx, t, r)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "FilterByState_ValidState",
			query:          "/workflows?State=REVOKED",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByState_InvalidState",
			query:          "/workflows?State=INVALID_STATE",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByArtifactType_ValidType",
			query:          "/workflows?ArtifactType=KEY",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "FilterByArtifactType_InvalidType",
			query:          "/workflows?ArtifactType=INVALID_TYPE",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByActionType_ValidType",
			query:          "/workflows?ActionType=UPDATE_STATE",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByActionType_InvalidType",
			query:          "/workflows?ActionType=INVALID_ACTION",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByMultipleParameters",
			query:          "/workflows?State=REVOKED&ArtifactType=KEY&ActionType=UPDATE_STATE",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "FilterByUserID",
			query:          "/workflows?UserID=d30fa7b3-1da4-483f-9f7c-64cd1b4678e5",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "FilterByInvlaidUserID",
			query:          "/workflows?UserID=invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "FilterByUserIDwithworkflows",
			query:          "/workflows?UserID=76e06743-80c6-4372-a195-269e4473036d",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := testutils.MakeHTTPRequest(t, sv, testutils.RequestOptions{
				Method:   http.MethodGet,
				Endpoint: tt.query,
				Tenant:   tenant,
			})

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				response := testutils.GetJSONBody[cmkapi.WorkflowList](t, w)

				assert.Len(t, response.Value, tt.expectedCount)
			}
		})
	}
}
