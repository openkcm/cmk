package operator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/samber/oops"
	"google.golang.org/protobuf/proto"

	goamqp "github.com/Azure/go-amqp"
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	tmdb "github.com/openkcm/cmk-core/internal/tenant-manager/db"
	"github.com/openkcm/cmk-core/utils/base62"
)

const (
	reconcileAfterSecProcessing int64 = 3
	reconcileAfterSecError      int64 = 15

	TaskCreateTenantLog    = "Handling Create Tenant task"
	TaskApplyTenantAuthLog = "Handling Apply Tenant Auth task"
	TaskTerminateTenantLog = "Handling Terminate Tenant task"
	TaskBlockTenantLog     = "Handling Block Tenant task"
	TaskUnblockTenantLog   = "Handling Unblock Tenant task"

	operatorComponent       = "operator"
	msgRegisteringHandler   = "registering handler"
	msgInitializingOperator = "initializing operator"

	WorkingStateTenantCreating            = "tenant is being created"
	WorkingStateTenantCreatedSuccessfully = "tenant created successfully"
	WorkingStateSchemaEncodingFailed      = "schema encoding failed"
	WorkingStateUnmarshallingFailed       = "failed to unmarshal tenant data"
	WorkingStateSchemaCreationFailed      = "schema creation failed"
	WorkingStateGroupsCreationFailed      = "group creation failed"
	WorkingStateSendingGroupsFailed       = "failed to send groups to registry"
	WorkingStateToBeImplemented           = "to be implemented"
)

type TenantOperator struct {
	db           *multitenancy.DB
	responder    *amqp.AMQP
	repo         repo.Repo
	tenantClient tenantgrpc.ServiceClient
}

func NewTenantOperator(
	db *multitenancy.DB,
	amqpClient *amqp.AMQP,
	tenantClient tenantgrpc.ServiceClient,
) (*TenantOperator, error) {
	if db == nil {
		return nil, oops.Errorf("db is nil")
	}

	if amqpClient == nil {
		return nil, oops.Errorf("amqpClient is nil")
	}

	if tenantClient == nil {
		return nil, oops.Errorf("tenantClient is nil")
	}

	r := sql.NewRepository(db)

	return &TenantOperator{
		db:           db,
		responder:    amqpClient,
		repo:         r,
		tenantClient: tenantClient,
	}, nil
}

// RunOperator initializes orbital operator and registers all the handlers
func (o *TenantOperator) RunOperator(ctx context.Context) error {
	// Initialize an orbital operator that uses the responder
	operator, err := orbital.NewOperator(o.responder)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgInitializingOperator)
	}

	// Register a handler for the Create Tenant task type
	err = operator.RegisterHandler(
		tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(),
		o.handleCreateTenant,
	)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgRegisteringHandler)
	}

	// Register a handler for the Block Tenant task type
	err = operator.RegisterHandler(
		tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(),
		handleBlockTenant,
	)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgRegisteringHandler)
	}

	// Register a handler for the Unblock Tenant task type
	err = operator.RegisterHandler(
		tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(),
		handleUnblockTenant,
	)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgRegisteringHandler)
	}

	// Register a handler for the TerminateTenant task type
	err = operator.RegisterHandler(
		tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String(),
		handleTerminateTenant,
	)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgRegisteringHandler)
	}

	// Register a handler for the JWT Announcement task type
	err = operator.RegisterHandler(
		tenantgrpc.ACTION_ACTION_APPLY_TENANT_AUTH.String(),
		handleApplyTenantAuth,
	)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgRegisteringHandler)
	}

	// Start the operator to listen for task requests and respond
	operator.ListenAndRespond(ctx)

	return nil
}

func WithMTLS(mtls commoncfg.MTLS) amqp.ClientOption {
	return func(o *goamqp.ConnOptions) error {
		tlsConfig, err := commoncfg.LoadMTLSConfig(&mtls)
		if err != nil {
			return errs.Wrap(config.ErrLoadMTLSConfig, err)
		}

		o.TLSConfig = tlsConfig
		o.SASLType = goamqp.SASLTypeExternal("")

		return nil
	}
}

func newHandlerResponse(
	ctx context.Context,
	state string,
	result orbital.Result,
	err error,
) (orbital.HandlerResponse, error) {
	reconcileAfter := reconcileAfterSecProcessing

	if err != nil {
		log.Error(ctx, "Task Failed", err, slog.String("State", state))

		if result == orbital.ResultProcessing {
			state = fmt.Sprintf("%s: %s", state, err.Error())
			reconcileAfter = reconcileAfterSecError
			err = nil // clear the error to avoid task termination by orbital
		}
	}

	resp := orbital.HandlerResponse{
		WorkingState:      []byte(state),
		Result:            result,
		ReconcileAfterSec: reconcileAfter,
	}

	return resp, err
}

// handleCreateTenant is handler for Create Tenant task
func (o *TenantOperator) handleCreateTenant(ctx context.Context, req orbital.HandlerRequest) (
	orbital.HandlerResponse, error,
) {
	// Step 1: Unmarshal tenant data
	tenant, err := unmarshalTenantData(ctx, req.Data)
	if err != nil {
		return newHandlerResponse(ctx, WorkingStateUnmarshallingFailed, orbital.ResultFailed, err)
	}

	ctx = log.InjectTenant(ctx, tenant)

	// Step 2: Check the tenant creation progress
	probe := &TenantProbe{
		MultitenancyDB: o.db,
		Repo:           o.repo,
	}

	probeResult, err := probe.Check(ctx, tenant)
	if err != nil {
		return newHandlerResponse(ctx, err.Error(), orbital.ResultProcessing, err)
	}

	// Step 3: If all steps completed, finalize tenant creation by sending user groups to registry
	if isProvisioningComplete(probeResult) {
		return o.finalizeTenantProvisioning(ctx, tenant.ID)
	}

	// Step 4: If schema creation is pending, create the schema
	if probeResult.SchemaStatus != SchemaExists {
		err = o.createTenantSchema(ctx, tenant)
		if err != nil {
			return newHandlerResponse(ctx, WorkingStateSchemaCreationFailed, orbital.ResultProcessing, err)
		}
	}

	// Step 5: If groups creation is pending (and schema is created), create the groups
	if probeResult.GroupsStatus != GroupsExist {
		err = o.createTenantGroups(ctx, tenant)
		if err != nil {
			return newHandlerResponse(ctx, WorkingStateGroupsCreationFailed, orbital.ResultProcessing, err)
		}
	}
	// Step 6: Return processing state, if no errors, to re-invoke the handler for finalization
	return newHandlerResponse(ctx, WorkingStateTenantCreating, orbital.ResultProcessing, nil)
}

// unmarshalTenantData extracts tenant data from the request payload, encodes schema name, and returns a Tenant model
func unmarshalTenantData(ctx context.Context, data []byte) (*model.Tenant, error) {
	tenantProto := &tenantgrpc.Tenant{}

	err := proto.Unmarshal(data, tenantProto)
	if err != nil {
		return nil, oops.Wrapf(err, WorkingStateUnmarshallingFailed)
	}

	encodedSchemaName, err := base62.EncodeSchemaNameBase62(tenantProto.GetId())
	if err != nil {
		log.Error(ctx, WorkingStateSchemaEncodingFailed, err)
		return nil, oops.Wrapf(err, WorkingStateSchemaEncodingFailed)
	}

	// Create a tenant model from the request data
	return &model.Tenant{
		ID:     tenantProto.GetId(),
		Region: tenantProto.GetRegion(),
		Status: model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
		TenantModel: multitenancy.TenantModel{
			DomainURL:  encodedSchemaName,
			SchemaName: encodedSchemaName,
		},
	}, nil
}

// isProvisioningComplete checks if both schema and groups existence checks are successful
func isProvisioningComplete(result TenantProbeResult) bool {
	return result.SchemaStatus == SchemaExists &&
		result.GroupsStatus == GroupsExist
}

// finalizeTenantProvisioning sends user groups to registry to complete tenant creation
func (o *TenantOperator) finalizeTenantProvisioning(
	ctx context.Context,
	tenantID string,
) (orbital.HandlerResponse, error) {
	success, err := o.sendTenantUserGroupsToRegistry(ctx, tenantID)
	log.Debug(ctx, "Attempted to send user groups to registry", slog.Bool("success", success))

	if err != nil || !success {
		return newHandlerResponse(ctx, WorkingStateSendingGroupsFailed, orbital.ResultProcessing, err)
	}

	return newHandlerResponse(ctx, WorkingStateTenantCreatedSuccessfully, orbital.ResultDone, nil)
}

// createTenantSchema creates the tenant schema in the database
func (o *TenantOperator) createTenantSchema(ctx context.Context, tenant *model.Tenant) error {
	err := tmdb.CreateSchema(ctx, o.db, tenant)
	if err != nil {
		if errors.Is(err, tmdb.ErrOnboardingInProgress) {
			log.Info(ctx, "Onboarding in progress, returning early")
			return nil
		}
	}

	return err
}

// createTenantGroups creates the tenant groups
func (o *TenantOperator) createTenantGroups(ctx context.Context, tenant *model.Tenant) error {
	err := tmdb.CreateDefaultGroups(ctx, tenant, o.repo)
	if err != nil {
		if errors.Is(err, tmdb.ErrOnboardingInProgress) {
			log.Info(ctx, "Onboarding in progress, returning early")
			return nil
		}
	}

	return err
}

// handleBlockTenant is handler for Block Tenant task
func handleBlockTenant(_ context.Context, _ orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return orbital.HandlerResponse{
		WorkingState: []byte(WorkingStateToBeImplemented),
		Result:       orbital.ResultFailed,
	}, nil
}

// handleUnblockTenant is handler for Unblock Tenant task
func handleUnblockTenant(_ context.Context, _ orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return orbital.HandlerResponse{
		WorkingState: []byte(WorkingStateToBeImplemented),
		Result:       orbital.ResultFailed,
	}, nil
}

// handleTerminateTenant is handler for Terminate Tenant task
func handleTerminateTenant(_ context.Context, _ orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return orbital.HandlerResponse{
		WorkingState: []byte(WorkingStateToBeImplemented),
		Result:       orbital.ResultFailed,
	}, nil
}

// Add handler for JWT Announcement task
func handleApplyTenantAuth(_ context.Context, _ orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	return orbital.HandlerResponse{
		WorkingState: []byte(WorkingStateToBeImplemented),
		Result:       orbital.ResultFailed,
	}, nil
}

// sendTenantUserGroupsToRegistry sends the user groups of a tenant to the Registry service
func (o *TenantOperator) sendTenantUserGroupsToRegistry(ctx context.Context, tenantID string) (bool, error) {
	groups := []string{
		model.NewIAMIdentifier(constants.TenantAdminGroup, tenantID),
		model.NewIAMIdentifier(constants.TenantAuditorGroup, tenantID),
	}
	req := &tenantgrpc.SetTenantUserGroupsRequest{
		Id:         tenantID,
		UserGroups: groups,
	}

	resp, err := o.tenantClient.SetTenantUserGroups(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.GetSuccess(), err
}
