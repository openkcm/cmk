package workflow_test

import (
	"database/sql"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/clients/registry/systems"
	"github.tools.sap/kms/cmk/internal/config"
	eventprocessor "github.tools.sap/kms/cmk/internal/event-processor"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	sqlRepo "github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	"github.tools.sap/kms/cmk/internal/workflow"
)

var (
	userID01     = "00000000-0000-0000-0000-000000000001"
	userID02     = "00000000-0000-0000-0000-000000000002"
	userID03     = "00000000-0000-0000-0000-000000000003"
	userID04     = "00000000-0000-0000-0000-000000000004"
	artifactID01 = uuid.MustParse("00000000-0000-0000-1111-000000000001")

	sqlNullBoolNull  = sql.NullBool{Bool: true, Valid: false}
	sqlNullBoolTrue  = sql.NullBool{Bool: true, Valid: true}
	sqlNullBoolFalse = sql.NullBool{Bool: false, Valid: true}
)

func SetupWorkflowManager(t *testing.T) (*manager.Manager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, dbConf := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Workflow{}, &model.WorkflowApprover{},
			&model.Key{},
			&model.KeyVersion{},
			&model.TenantConfig{},
			&model.KeyConfiguration{},
			&model.System{},
			&model.SystemProperty{},
			&model.ImportParams{},
			&model.KeystoreConfiguration{},
			&model.Certificate{},
		},
		CreateDatabase: true,
	})
	cfg := config.Config{
		Plugins:  testutils.SetupMockPlugins(testutils.KeyStorePlugin, testutils.CertIssuer),
		Database: dbConf,
	}
	tenant := tenants[0]
	ctx := testutils.CreateCtxWithTenant(tenant)

	ctlg, err := catalog.New(ctx, &cfg)
	assert.NoError(t, err)

	logger := testutils.SetupLoggerWithBuffer()

	systemService := systems.NewFakeService(logger)
	_, grpcClient := testutils.NewGRPCSuite(t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
		},
	)

	clientsFactory, err := clients.NewFactory(config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: grpcClient.Target(),
			SecretRef: &commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
		},
	})

	assert.NoError(t, err)
	assert.NoError(t, clientsFactory.Close())

	r := sqlRepo.NewRepository(db)
	reconciler, err := eventprocessor.NewCryptoReconciler(ctx, &cfg, r, ctlg, clientsFactory)
	assert.NoError(t, err)

	ksConfig := testutils.NewKeystoreConfig(func(_ *model.KeystoreConfiguration) {})
	keystoreDefaultCert := testutils.NewCertificate(func(c *model.Certificate) {
		c.Purpose = model.CertificatePurposeKeystoreDefault
		c.CommonName = testutils.TestDefaultKeystoreCommonName
	})
	testutils.CreateTestEntities(ctx, t, r, ksConfig, keystoreDefaultCert)

	assert.NoError(t, err)

	return manager.New(ctx, r, &cfg, clientsFactory, ctlg, reconciler, nil), db, tenants[0]
}

func TestWorkflowLifecycleTransitions(t *testing.T) {
	wfMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:          uuid.New(),
			State:       workflow.StateInitial.String(),
			InitiatorID: userID01,
			Approvers: []model.WorkflowApprover{
				{UserID: userID02, Approved: sqlNullBoolNull},
				{UserID: userID03, Approved: sqlNullBoolNull},
			},
			ArtifactType: workflow.ArtifactTypeKey.String(),
			ArtifactID:   artifactID01,
			ActionType:   workflow.ActionTypeUpdateState.String(),
			Parameters:   "DISABLED",
		}
	})

	tests := []struct {
		name          string
		workflow      model.Workflow
		actorID       string
		transition    workflow.Transition
		expectErr     bool
		errMessage    string         // If expectErr is true, this is the expected error message
		expectedState workflow.State // If expectErr is false, this is the expected state after the transition
	}{
		{
			name: "create from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionCreate,
			expectErr:     false,
			expectedState: workflow.StateWaitApproval,
		},
		{
			name: "create from initial not enough approvers",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID02, Approved: sqlNullBoolNull},
				}
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "insufficient approvers to transition to next state: 1, required: 2",
		},
		{
			name: "revoke from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state INITIAL",
		},
		{
			name: "reject from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state INITIAL",
		},
		{
			name: "approve from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state INITIAL",
		},
		{
			name: "confirm from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state INITIAL",
		},
		{
			name: "expire from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "execute from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from initial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: " +
				"event CREATE inappropriate in current state REVOKED",
		},
		{
			name: "approve from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: " +
				"event APPROVE inappropriate in current state REVOKED",
		},
		{
			name: "reject from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: " +
				"event REJECT inappropriate in current state REVOKED",
		},
		{
			name: "revoke from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: " +
				"event REVOKE inappropriate in current state REVOKED",
		},
		{
			name: "expire from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: " +
				"event CONFIRM inappropriate in current state REVOKED",
		},
		{
			name: "execute from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from revoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: event CREATE inappropriate in current state REJECTED",
		},
		{
			name: "approve from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state REJECTED",
		},
		{
			name: "reject from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state REJECTED",
		},
		{
			name: "revoke from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state REJECTED",
		},
		{
			name: "expire from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state REJECTED",
		},
		{
			name: "execute from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: event CREATE inappropriate in current state EXPIRED",
		},
		{
			name: "approve from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state EXPIRED",
		},
		{
			name: "reject from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state EXPIRED",
		},
		{
			name: "revoke from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state EXPIRED",
		},
		{
			name: "expire from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state EXPIRED",
		},
		{
			name: "execute from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from expired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: " +
				"event CREATE inappropriate in current state WAIT_APPROVAL",
		},
		{
			name: "revoke from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionRevoke,
			expectErr:     false,
			expectedState: workflow.StateRevoked,
		},
		{
			name: "approve from wait approval not final",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:       userID02,
			transition:    workflow.TransitionApprove,
			expectErr:     false,
			expectedState: workflow.StateWaitApproval,
		},
		{
			name: "approve from wait approval final",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID02, Approved: sqlNullBoolNull},
					{UserID: userID03, Approved: sqlNullBoolTrue},
				}
			}),
			actorID:       userID02,
			transition:    workflow.TransitionApprove,
			expectErr:     false,
			expectedState: workflow.StateWaitConfirmation,
		},
		{
			name: "approve from wait approval reach threshold",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID01, Approved: sqlNullBoolNull},
					{UserID: userID02, Approved: sqlNullBoolNull},
					{UserID: userID03, Approved: sqlNullBoolTrue},
				}
			}),
			actorID:       userID02,
			transition:    workflow.TransitionApprove,
			expectErr:     false,
			expectedState: workflow.StateWaitConfirmation,
		},
		{
			name: "reject from wait approval first rejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID01, Approved: sqlNullBoolNull},
					{UserID: userID02, Approved: sqlNullBoolNull},
					{UserID: userID03, Approved: sqlNullBoolNull},
					{UserID: userID04, Approved: sqlNullBoolNull},
				}
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  false,
			// Still in wait approval as other 3 approvers can still approve to meet threshold (2)
			expectedState: workflow.StateWaitApproval,
		},
		{
			name: "reject from wait approval early rejected, impossible to approve",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID01, Approved: sqlNullBoolNull},
					{UserID: userID02, Approved: sqlNullBoolNull},
					{UserID: userID03, Approved: sqlNullBoolNull},
				}
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  false,
			// Now rejected as even if all others approve, threshold (2) cannot be met
			expectedState: workflow.StateRejected,
		},
		{
			name: "approve from wait approval not approver",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID02, Approved: sqlNullBoolTrue},
				}
			}),
			actorID:    userID01,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "invalid event actor: " +
				"user 00000000-0000-0000-0000-000000000001 is not the approver of the workflow",
		},
		{
			name: "confirm from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: " +
				"event CONFIRM inappropriate in current state WAIT_APPROVAL",
		},
		{
			name: "reject from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID02, Approved: sqlNullBoolFalse},
					{UserID: userID03, Approved: sqlNullBoolTrue},
				}
			}),
			actorID:       userID02,
			transition:    workflow.TransitionReject,
			expectErr:     false,
			expectedState: workflow.StateRejected,
		},
		{
			name: "reject from wait approval not approver",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
				// Set all approvers
				wf.Approvers = []model.WorkflowApprover{
					{UserID: userID02, Approved: sqlNullBoolFalse},
				}
			}),
			actorID:    userID01,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "invalid event actor: " +
				"user 00000000-0000-0000-0000-000000000001 is not the approver of the workflow",
		},
		{
			name: "expire from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "execute from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from wait approval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: " +
				"event CREATE inappropriate in current state WAIT_CONFIRMATION",
		},
		{
			name: "revoke from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionRevoke,
			expectErr:     false,
			expectedState: workflow.StateRevoked,
		},
		{
			name: "approve from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: " +
				"event APPROVE inappropriate in current state WAIT_CONFIRMATION",
		},
		{
			name: "reject from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: " +
				"event REJECT inappropriate in current state WAIT_CONFIRMATION",
		},
		{
			name: "expire from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateSuccessful,
		},
		{
			name: "confirm from wait confirmation wrong artifact type",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.ArtifactType = "SOMETHING"
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateFailed,
		},
		{
			name: "confirm from wait confirmation wrong action type",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.ActionType = "DOSTUFF"
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateFailed,
		},
		{
			name: "confirm from wait confirmation wrong parameters",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
				wf.Parameters = "WRONG"
			}),
			actorID:       userID01,
			transition:    workflow.TransitionConfirm,
			expectErr:     false,
			expectedState: workflow.StateFailed,
		},
		{
			name: "confirm from wait confirmation wrong as approver",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "invalid event actor: " +
				"user 00000000-0000-0000-0000-000000000002 is not the initiator of the workflow",
		},
		{
			name: "execute from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from wait confirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: event CREATE inappropriate in current state EXECUTING",
		},
		{
			name: "approve from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state EXECUTING",
		},
		{
			name: "reject from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state EXECUTING",
		},
		{
			name: "revoke from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state EXECUTING",
		},
		{
			name: "expire from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state EXECUTING",
		},
		{
			name: "execute from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from executing",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: event CREATE inappropriate in current state SUCCESSFUL",
		},
		{
			name: "approve from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state SUCCESSFUL",
		},
		{
			name: "reject from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state SUCCESSFUL",
		},
		{
			name: "revoke from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state SUCCESSFUL",
		},
		{
			name: "expire from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state SUCCESSFUL",
		},
		{
			name: "execute from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from successful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "create from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionCreate,
			expectErr:  true,
			errMessage: "failed to execute transition CREATE: event CREATE inappropriate in current state FAILED",
		},
		{
			name: "approve from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionApprove,
			expectErr:  true,
			errMessage: "failed to execute transition APPROVE: event APPROVE inappropriate in current state FAILED",
		},
		{
			name: "reject from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID02,
			transition: workflow.TransitionReject,
			expectErr:  true,
			errMessage: "failed to execute transition REJECT: event REJECT inappropriate in current state FAILED",
		},
		{
			name: "revoked from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionRevoke,
			expectErr:  true,
			errMessage: "failed to execute transition REVOKE: event REVOKE inappropriate in current state FAILED",
		},
		{
			name: "expire from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExpire,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "confirm from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionConfirm,
			expectErr:  true,
			errMessage: "failed to execute transition CONFIRM: event CONFIRM inappropriate in current state FAILED",
		},
		{
			name: "execute from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionExecute,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
		{
			name: "fail from failed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			actorID:    userID01,
			transition: workflow.TransitionFail,
			expectErr:  true,
			errMessage: "automated transition cannot be triggered by user input",
		},
	}

	mgr, db, tenant := SetupWorkflowManager(t)
	r := sqlRepo.NewRepository(db)

	ctx := testutils.CreateCtxWithTenant(tenant)
	keyConf := &model.KeyConfiguration{ID: uuid.New(), AdminGroup: *testutils.NewGroup(func(_ *model.Group) {}),
		CreatorID: uuid.NewString()}
	err := r.Create(ctx, keyConf)
	assert.NoError(t, err)

	err = r.Create(
		ctx,
		&model.Key{
			ID:                 artifactID01,
			Provider:           "AWS",
			KeyType:            "SYSTEM_MANAGED",
			KeyConfigurationID: keyConf.ID,
		},
	)
	assert.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare
			err := r.Create(ctx, &tt.workflow)
			assert.NoError(t, err)

			// Act
			lifecycle := workflow.NewLifecycle(&tt.workflow, mgr.Keys, mgr.KeyConfig, mgr.System, r, tt.actorID, 2)
			transitionErr := lifecycle.ApplyTransition(ctx, tt.transition)

			// Verify
			// Retrieve workflow from database again to get most up-to-date representation
			wf := &model.Workflow{ID: tt.workflow.ID}
			ok, retrievalErr := r.First(ctx, wf, *repo.NewQuery())
			assert.NoError(t, retrievalErr)

			if tt.expectErr {
				assert.Error(t, transitionErr)
				assert.EqualError(t, transitionErr, tt.errMessage)
			} else {
				assert.NoError(t, transitionErr)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedState.String(), wf.State)
			}
		})
	}
}

func TestWorkflowLifecycleExpiration(t *testing.T) {
	wfMutator := testutils.NewMutator(func() model.Workflow {
		return model.Workflow{
			ID:          uuid.New(),
			State:       workflow.StateInitial.String(),
			InitiatorID: userID01,
			Approvers: []model.WorkflowApprover{
				{UserID: userID02, Approved: sqlNullBoolNull},
				{UserID: userID03, Approved: sqlNullBoolNull},
			},
			ArtifactType: workflow.ArtifactTypeKey.String(),
			ArtifactID:   artifactID01,
			ActionType:   workflow.ActionTypeUpdateState.String(),
			Parameters:   "DISABLED",
		}
	})
	tests := []struct {
		name       string
		workflow   model.Workflow
		expectErr  bool
		errMessage string // If expectErr is true, this is the expected error message
	}{
		{
			name: "TestWorkflowLifecycleExpiration_FromInitial",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateInitial.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state INITIAL",
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromRevoked",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRevoked.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state REVOKED",
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromRejected",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateRejected.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state REJECTED",
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromExpired",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExpired.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state EXPIRED",
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromWaitApproval",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitApproval.String()
			}),
			expectErr: false,
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromWaitConfirmation",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateWaitConfirmation.String()
			}),
			expectErr: false,
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromExecuting",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateExecuting.String()
			}),
			expectErr: false,
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromSuccessful",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateSuccessful.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state SUCCESSFUL",
		},
		{
			name: "TestWorkflowLifecycleExpiration_FromFailed",
			workflow: wfMutator(func(wf *model.Workflow) {
				wf.State = workflow.StateFailed.String()
			}),
			expectErr:  true,
			errMessage: "failed to execute transition EXPIRE: event EXPIRE inappropriate in current state FAILED",
		},
	}

	mgr, db, tenant := SetupWorkflowManager(t)
	r := sqlRepo.NewRepository(db)

	ctx := testutils.CreateCtxWithTenant(tenant)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare
			err := r.Create(ctx, &tt.workflow)
			assert.NoError(t, err)

			// Act
			lifecycle := workflow.NewLifecycle(&tt.workflow, mgr.Keys, mgr.KeyConfig, mgr.System, r, userID01, 2)
			transitionErr := lifecycle.Expire(ctx)

			// Verify
			// Retrieve workflow from database again to get most up-to-date representation
			wf := &model.Workflow{ID: tt.workflow.ID}
			ok, retrievalErr := r.First(ctx, wf, *repo.NewQuery())
			assert.NoError(t, retrievalErr)

			if tt.expectErr {
				assert.Error(t, transitionErr)
				assert.EqualError(t, transitionErr, tt.errMessage)
			} else {
				assert.NoError(t, transitionErr)
				assert.True(t, ok)
				assert.Equal(t, workflow.StateExpired.String(), wf.State)
			}
		})
	}
}
