package workflow_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	sqlRepo "github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/workflow"
)

func TestWorkflowKeyConfigActions(t *testing.T) {
	mgr, db, tenant := SetupWorkflowManager(t)
	r := sqlRepo.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	tests := []struct {
		name          string
		workflow      func(k *model.Key) *model.Workflow
		transition    workflow.Transition
		expectedState workflow.State
	}{
		{
			name: "Delete key config",
			workflow: func(k *model.Key) *model.Workflow {
				return testutils.NewWorkflow(func(wf *model.Workflow) {
					wf.State = workflow.StateWaitConfirmation.String()
					wf.ActionType = workflow.ActionTypeUpdatePrimary.String()
					wf.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
					wf.Approvers = []model.WorkflowApprover{
						*testutils.NewWorkflowApprover(func(a *model.WorkflowApprover) {
							a.Approved = sqlNullBoolNull
						}),
						*testutils.NewWorkflowApprover(func(a *model.WorkflowApprover) {
							a.Approved = sqlNullBoolNull
						}),
					}
					wf.Parameters = k.ID.String()
				})
			},
			transition:    workflow.TransitionConfirm,
			expectedState: workflow.StateSuccessful,
		},
		{
			name: "Update primary key",
			workflow: func(k *model.Key) *model.Workflow {
				return testutils.NewWorkflow(func(wf *model.Workflow) {
					wf.State = workflow.StateWaitConfirmation.String()
					wf.ActionType = workflow.ActionTypeUpdatePrimary.String()
					wf.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
					wf.Approvers = []model.WorkflowApprover{
						*testutils.NewWorkflowApprover(func(a *model.WorkflowApprover) {
							a.Approved = sqlNullBoolNull
						}),
						*testutils.NewWorkflowApprover(func(a *model.WorkflowApprover) {
							a.Approved = sqlNullBoolNull
						}),
					}
					wf.Parameters = k.ID.String()
				})
			},
			transition:    workflow.TransitionConfirm,
			expectedState: workflow.StateSuccessful,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyConfig := testutils.NewKeyConfig(func(_ *model.KeyConfiguration) {})
			err := r.Create(ctx, keyConfig)
			assert.NoError(t, err)

			ctx := testutils.InjectClientDataIntoContext(
				ctx,
				uuid.NewString(),
				[]string{keyConfig.AdminGroup.IAMIdentifier},
			)

			key := testutils.NewKey(func(k *model.Key) {
				k.KeyConfigurationID = keyConfig.ID
			})
			err = r.Create(ctx, key)
			assert.NoError(t, err)

			wf := tt.workflow(key)

			err = r.Create(ctx, wf)
			assert.NoError(t, err)

			lifecycle := workflow.NewLifecycle(wf, mgr.Keys, mgr.KeyConfig, mgr.System, r, wf.InitiatorID, 2)
			err = lifecycle.ApplyTransition(ctx, tt.transition)
			assert.NoError(t, err)

			wf = &model.Workflow{ID: wf.ID}
			ok, err := r.First(ctx, wf, *repo.NewQuery())
			assert.NoError(t, err)
			assert.True(t, ok)

			keyConfig = &model.KeyConfiguration{ID: keyConfig.ID}
			ok, err = r.First(ctx, keyConfig, *repo.NewQuery())
			assert.True(t, ok)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedState.String(), wf.State)
		})
	}
}
