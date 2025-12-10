package workflow_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	sqlRepo "github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	"github.tools.sap/kms/cmk/internal/workflow"
)

var (
	systemID01    = uuid.MustParse("00000000-0000-0000-9999-000000000001")
	keyConfigID03 = uuid.MustParse("00000000-0000-0000-2222-000000000003")
	keyConfigID04 = uuid.MustParse("00000000-0000-0000-4222-000000000004")
	keyID03       = uuid.MustParse("00000000-0000-0000-3333-000000000003")
	keyID04       = uuid.MustParse("00000000-0000-0000-3333-000000000004")
)

func TestWorkflowSystemUpdateKeyConfiguration(t *testing.T) {
	wfMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:          systemID01,
			State:       workflow.StateInitial.String(),
			InitiatorID: userID01,
			Approvers: []model.WorkflowApprover{
				{UserID: userID02, Approved: sqlNullBoolNull},
				{UserID: userID03, Approved: sqlNullBoolNull},
			},
			ArtifactType: workflow.ArtifactTypeSystem.String(),
			ArtifactID:   systemID01,
			ActionType:   workflow.ActionTypeLink.String(),
			Parameters:   keyConfigID03.String(),
		}
	})
	tests := []struct {
		name          string
		workflow      model.Workflow
		actorID       string
		transition    workflow.Transition
		expectErr     bool
		errMessage    string
		expectedState workflow.State
	}{
		{
			name: "workflow system link",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateSuccessful,
		},
		{
			name: "workflow system switch",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.ActionType = workflow.ActionTypeSwitch.String()
				wf.Parameters = keyConfigID04.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateSuccessful,
		},
		{
			name: "workflow system unlink",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.ActionType = workflow.ActionTypeUnlink.String()
				wf.Parameters = ""
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateSuccessful,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, db, tenant := SetupWorkflowManager(t)
			r := sqlRepo.NewRepository(db)
			ctx := testutils.CreateCtxWithTenant(tenant)

			// Prepare
			err := r.Create(ctx, &tt.workflow)
			assert.NoError(t, err)

			key3 := testutils.NewKey(func(k *model.Key) {
				k.ID = keyID03
				k.KeyConfigurationID = keyConfigID04
			})

			key4 := testutils.NewKey(func(k *model.Key) {
				k.ID = keyID04
				k.KeyConfigurationID = keyConfigID04
			})

			keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
				c.ID = keyConfigID04
				c.PrimaryKeyID = &key4.ID
			})

			keyConfig3 := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
				c.ID = keyConfigID03
				c.PrimaryKeyID = &key3.ID
			})

			ctx = testutils.InjectClientDataIntoContext(
				ctx,
				uuid.NewString(),
				[]string{
					keyConfig3.AdminGroup.IAMIdentifier,
					keyConfig.AdminGroup.IAMIdentifier,
				},
			)

			testutils.CreateTestEntities(ctx, t, r, key3, key4, keyConfig, keyConfig3)

			system := &model.System{ID: systemID01, KeyConfigurationID: &keyConfigID04}
			err = r.Create(ctx, system)
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

			systemRet := &model.System{ID: systemID01, KeyConfigurationID: &keyConfigID03}
			ok, err = r.First(ctx, systemRet, *repo.NewQuery())
			assert.NoError(t, err)
			assert.True(t, ok)

			assert.NoError(t, transitionErr)
			assert.Equal(t, tt.expectedState.String(), wf.State)
		})
	}
}
