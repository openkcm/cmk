package workflow_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	sqlRepo "github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/workflow"
)

func TestKeyActionPrimary(t *testing.T) {
	wfMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:          keyConfigID01,
			State:       workflow.StateInitial.String(),
			InitiatorID: userID01,
			Approvers: []model.WorkflowApprover{
				{UserID: userID02, Approved: sqlNullBoolNull},
				{UserID: userID03, Approved: sqlNullBoolNull},
			},
			ArtifactType: workflow.ArtifactTypeKey.String(),
			ArtifactID:   keyConfigID01,
			ActionType:   workflow.ActionTypeUpdatePrimaryKey.String(),
			Parameters:   keyID02.String(),
		}
	})
	tests := []struct {
		name          string
		workflow      model.Workflow
		actorID       uuid.UUID
		transition    workflow.Transition
		expectErr     bool
		errMessage    string
		expectedState workflow.State
	}{
		{
			name: "workflow keyconfiguration update primary key",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.ActionType = workflow.ActionTypeUpdatePrimaryKey.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateSuccessful,
		},
	}

	for _, tt := range tests {
		mgr, db, tenant := SetupWorkflowManager(t)
		r := sqlRepo.NewRepository(db)

		ctx := testutils.CreateCtxWithTenant(tenant)

		t.Run(tt.name, func(t *testing.T) {
			// Prepare
			err := r.Create(ctx, &tt.workflow)
			assert.NoError(t, err)

			keyConf := &model.KeyConfiguration{ID: keyConfigID01, AdminGroup: model.Group{ID: uuid.New()}}
			err = r.Create(ctx, keyConf)
			assert.NoError(t, err)

			err = r.Create(ctx, &model.Key{
				ID:                 keyID01,
				Name:               uuid.NewString(),
				KeyConfigurationID: keyConfigID01,
				KeyType:            constants.KeyTypeSystemManaged,
			})
			assert.NoError(t, err)

			err = r.Create(
				ctx,
				&model.Key{
					ID:                 keyID02,
					Name:               uuid.NewString(),
					KeyConfigurationID: keyConfigID01,
					KeyType:            constants.KeyTypeSystemManaged,
				},
			)
			assert.NoError(t, err)

			// Act
			lifecycle := workflow.NewLifecycle(&tt.workflow, mgr.Keys, mgr.KeyConfig, mgr.System, r, tt.actorID, 2)
			transitionErr := lifecycle.ApplyTransition(ctx, tt.transition)

			// Verify
			// Retrieve workflow and other resources from database again to get most up-to-date representation
			wf := &model.Workflow{ID: tt.workflow.ID}
			ok, retrievalErr := r.First(ctx, wf, *repo.NewQuery())
			assert.NoError(t, retrievalErr)
			assert.True(t, ok)

			keyConfig := &model.KeyConfiguration{ID: keyConfigID01, PrimaryKeyID: &keyID02}
			ok, err = r.First(ctx, keyConfig, *repo.NewQuery())
			assert.True(t, ok)
			assert.NoError(t, err)

			assert.NoError(t, transitionErr)
			assert.Equal(t, tt.expectedState.String(), wf.State)
		})
	}
}
