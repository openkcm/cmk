package operator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/samber/oops"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	goamqp "github.com/Azure/go-amqp"
	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/utils/base62"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	reconcileAfterSecProcessing uint64 = 3
	reconcileAfterSecError      uint64 = 15

	operatorComponent       = "operator"
	msgRegisteringHandler   = "registering handler"
	msgInitializingOperator = "initializing operator"

	WorkingStateTenantCreating            = "tenant is being created"
	WorkingStateTenantCreatedSuccessfully = "tenant created successfully"
	WorkingStateSchemaEncodingFailed      = "schema encoding failed"
	WorkingStateUnmarshallingFailed       = "failed to unmarshal data"
	WorkingStateSchemaCreationFailed      = "schema creation failed"
	WorkingStateGroupsCreationFailed      = "group creation failed"
	WorkingStateSendingGroupsFailed       = "failed to send groups to registry"
)

var (
	ErrInvalidData       = errors.New("invalid data")
	ErrFailedResponse    = errors.New("failed response")
	ErrTenantOffboarding = errors.New("tenant offboarding error")

	ErrInvalidTenantID  = errors.New("invalid tenant ID")
	ErrInvalidAuthProps = errors.New("invalid authentication properties")
	ErrFailedApplyOIDC  = errors.New("failed apply OIDC")
)

type TenantOperator struct {
	db             *multitenancy.DB
	operatorTarget orbital.TargetOperator
	repo           repo.Repo
	clientsFactory clients.Factory
	gm             *manager.GroupManager
	tm             manager.Tenant
}

func NewTenantOperator(
	db *multitenancy.DB,
	operatorTarget orbital.TargetOperator,
	clientsFactory clients.Factory,
	tenantManager manager.Tenant,
	groupManager *manager.GroupManager,
) (*TenantOperator, error) {
	if db == nil {
		return nil, oops.Errorf("db is nil")
	}

	if operatorTarget.Client == nil {
		return nil, oops.Errorf("operator target client is nil")
	}

	if clientsFactory == nil {
		return nil, oops.Errorf("clients factory is nil")
	}

	if clientsFactory.Registry().Tenant() == nil {
		return nil, oops.Errorf("tenantClient is nil")
	}

	if clientsFactory.SessionManager().OIDCMapping() == nil {
		return nil, oops.Errorf("sessionManagerClient is nil")
	}

	r := sql.NewRepository(db)

	return &TenantOperator{
		db:             db,
		operatorTarget: operatorTarget,
		repo:           r,
		clientsFactory: clientsFactory,
		gm:             groupManager,
		tm:             tenantManager,
	}, nil
}

// RunOperator initializes the Orbital operator, registers all task handlers, and starts the listener.
// It returns a channel that is closed when the listener goroutine exits, or an error if initialization fails.
func (o *TenantOperator) RunOperator(ctx context.Context) error {
	// Initialize an orbital operator that uses the operator target
	operator, err := orbital.NewOperator(o.operatorTarget)
	if err != nil {
		return oops.In(operatorComponent).
			Wrapf(err, msgInitializingOperator)
	}

	// Register all handlers
	err = o.registerHandlers(operator)
	if err != nil {
		return err
	}

	log.Info(ctx, "Tenant Manager is running and waiting for tenant operations")

	// Start listener in goroutine
	go operator.ListenAndRespond(ctx)

	// Block until context is cancelled
	<-ctx.Done()
	log.Info(ctx, "Shutting down Tenant Manager due to context cancellation")

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

// handleCreateTenant is handler for Create Tenant task
func (o *TenantOperator) handleCreateTenant(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) error {
	// Step 1: Unmarshal tenant data
	tenant, err := unmarshalTenantData(ctx, req.Data)
	if err != nil {
		return newHandlerResponse(ctx, WorkingStateUnmarshallingFailed, orbital.ResultFailed, resp, err)
	}

	ctx = model.LogInjectTenant(ctx, tenant)

	// Step 2: Check the tenant creation progress
	probe := &TenantProbe{
		MultitenancyDB: o.db,
		Repo:           o.repo,
	}

	probeResult, err := probe.Check(ctx, tenant)
	if err != nil {
		return newHandlerResponse(ctx, err.Error(), orbital.ResultProcessing, resp, err)
	}

	// Step 3: If all steps completed, finalize tenant creation by sending user groups to registry
	if isProvisioningComplete(probeResult) {
		return o.finalizeTenantProvisioning(ctx, tenant.ID, resp)
	}

	// Step 4: If schema creation is pending, create the schema
	if probeResult.SchemaStatus != SchemaExists {
		err = o.createTenantSchema(ctx, tenant)
		if err != nil {
			return newHandlerResponse(ctx, WorkingStateSchemaCreationFailed, orbital.ResultProcessing, resp, err)
		}
	}

	// Step 5: If groups creation is pending (and schema is created), create the groups
	if probeResult.GroupsStatus != GroupsExist {
		err = o.createTenantGroups(ctx, tenant)
		if err != nil {
			return newHandlerResponse(ctx, WorkingStateGroupsCreationFailed, orbital.ResultProcessing, resp, err)
		}
	}
	// Step 6: Return processing state, if no errors, to re-invoke the handler for finalization
	return newHandlerResponse(ctx, WorkingStateTenantCreating, orbital.ResultProcessing, resp, nil)
}

// handleApplyTenantAuth is handler for the Apply Tenant Auth task.
func (o *TenantOperator) handleApplyTenantAuth(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) error {
	authProto := &authgrpc.Auth{}

	err := proto.Unmarshal(req.Data, authProto)
	if err != nil {
		return errs.Wrap(ErrInvalidData, err)
	}

	tenantID := authProto.GetTenantId()
	if tenantID == "" {
		return ErrInvalidTenantID
	}

	ctx = slogctx.With(ctx, "tenantID", tenantID)

	oidcConfig, err := extractOIDCConfig(authProto.GetProperties())
	if err != nil {
		return errs.Wrap(ErrInvalidAuthProps, err)
	}

	err = o.applyOIDC(ctx, tenantID, oidcConfig)
	if errors.Is(err, ErrFailedApplyOIDC) {
		return err
	}

	if err != nil {
		slogctx.Error(ctx, "error while applying OIDC", "error", err)

		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecError

		return nil
	}

	resp.Result = orbital.ResultDone

	return nil
}

// applyOIDC applies the OIDC configuration to the tenant by updating the issuer URL
// and sending an ApplyOIDCMapping request to the Session Manager service.
func (o *TenantOperator) applyOIDC(ctx context.Context, tenantID string, cfg OIDCConfig) error {
	return o.repo.Transaction(ctx, func(ctx context.Context) error {
		success, err := o.repo.Patch(ctx, &model.Tenant{
			ID:        tenantID,
			IssuerURL: cfg.Issuer,
		}, *repo.NewQuery().UpdateAll(false))
		if err != nil {
			return err
		}

		if !success {
			return errs.Wrapf(ErrFailedApplyOIDC, "could not update tenant issuer URL in database")
		}

		resp, err := o.clientsFactory.SessionManager().OIDCMapping().ApplyOIDCMapping(
			ctx,
			&oidcmappinggrpc.ApplyOIDCMappingRequest{
				TenantId:   tenantID,
				Issuer:     cfg.Issuer,
				JwksUri:    &cfg.JwksURI,
				Audiences:  cfg.Audiences,
				Properties: cfg.AdditionalProperties,
			},
		)
		if err != nil {
			return err
		}

		if !resp.GetSuccess() {
			return errs.Wrapf(ErrFailedApplyOIDC, resp.GetMessage())
		}

		return nil
	})
}

// handleBlockTenant is handler for Block Tenant task
func (o *TenantOperator) handleBlockTenant(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) error {
	tenantProto := &tenantgrpc.Tenant{}

	err := proto.Unmarshal(req.Data, tenantProto)
	if err != nil {
		return errs.Wrap(ErrInvalidData, err)
	}

	grpcResp, err := o.clientsFactory.SessionManager().OIDCMapping().BlockOIDCMapping(
		ctx,
		&oidcmappinggrpc.BlockOIDCMappingRequest{
			TenantId: tenantProto.GetId(),
		},
	)
	//nolint:nilerr
	if err != nil {
		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecError
		return nil
	}

	if !grpcResp.GetSuccess() {
		return errs.Wrapf(ErrFailedResponse, "session manager could not block OIDC mapping")
	}

	resp.Result = orbital.ResultDone

	return nil
}

// handleUnblockTenant is handler for Unblock Tenant task
func (o *TenantOperator) handleUnblockTenant(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) error {
	tenantProto := &tenantgrpc.Tenant{}

	err := proto.Unmarshal(req.Data, tenantProto)
	if err != nil {
		return errs.Wrap(ErrInvalidData, err)
	}

	grpcResp, err := o.clientsFactory.SessionManager().OIDCMapping().UnblockOIDCMapping(
		ctx,
		&oidcmappinggrpc.UnblockOIDCMappingRequest{
			TenantId: tenantProto.GetId(),
		},
	)
	//nolint:nilerr
	if err != nil {
		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecError
		return nil
	}

	if !grpcResp.GetSuccess() {
		return errs.Wrapf(ErrFailedResponse, "session manager could not unblock OIDC mapping")
	}

	resp.Result = orbital.ResultDone

	return nil
}

// handleTerminateTenant is handler for Terminate Tenant task
//
//nolint:cyclop, funlen
func (o *TenantOperator) handleTerminateTenant(
	ctx context.Context,
	req orbital.HandlerRequest,
	resp *orbital.HandlerResponse,
) error {
	tenantProto := &tenantgrpc.Tenant{}

	err := proto.Unmarshal(req.Data, tenantProto)
	if err != nil {
		return errs.Wrap(ErrInvalidData, err)
	}

	ctx = slogctx.With(ctx, "tenantID", tenantProto.GetId())

	grpcResp, err := o.clientsFactory.SessionManager().OIDCMapping().RemoveOIDCMapping(
		ctx,
		&oidcmappinggrpc.RemoveOIDCMappingRequest{
			TenantId: tenantProto.GetId(),
		},
	)
	st, ok := status.FromError(err)
	if !ok {
		log.Error(ctx, "failed getting info on sessionManager error", err)
	}
	if st.Code() == codes.Internal {
		log.Error(ctx, "removeOIDC failed with internal err", err)
	}
	if err != nil && st.Code() != codes.Internal {
		log.Error(ctx, "error while removing OIDC mapping", err)

		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecError

		return nil
	}

	if !grpcResp.GetSuccess() {
		return errs.Wrapf(ErrFailedResponse, "session manager could not remove OIDC mapping")
	}

	result, err := o.terminateTenant(ctx, tenantProto.GetId())
	if err != nil {
		log.Error(ctx, "error while terminating tenant", err)

		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecError

		return nil
	}

	switch result.Status {
	case manager.OffboardingFailed:
		return ErrTenantOffboarding
	case manager.OffboardingProcessing:
		resp.Result = orbital.ResultProcessing
		resp.ReconcileAfterSec = reconcileAfterSecProcessing
	case manager.OffboardingSuccess:
		resp.Result = orbital.ResultDone
	default:
		return errs.Wrapf(ErrTenantOffboarding, "unknown offboarding status")
	}

	return nil
}

func (o *TenantOperator) terminateTenant(ctx context.Context, tenantID string) (manager.OffboardingResult, error) {
	tenantCtx := cmkcontext.CreateTenantContext(ctx, tenantID)

	result, err := o.tm.OffboardTenant(tenantCtx)
	if err != nil {
		return manager.OffboardingResult{}, err
	}

	if result.Status != manager.OffboardingSuccess {
		return result, nil
	}

	err = o.tm.DeleteTenant(tenantCtx)
	if err != nil {
		return manager.OffboardingResult{}, err
	}

	return result, nil
}

func newHandlerResponse(
	ctx context.Context,
	state string,
	result orbital.Result,
	resp *orbital.HandlerResponse,
	err error,
) error {
	reconcileAfter := reconcileAfterSecProcessing

	if err != nil {
		log.Error(ctx, "Task Failed", err, slog.String("State", state))

		if result == orbital.ResultProcessing {
			state = fmt.Sprintf("%s: %s", state, err.Error())
			reconcileAfter = reconcileAfterSecError
			err = nil // clear the error to avoid task termination by orbital
		}
	}

	newHandlerResponse := orbital.HandlerResponse{
		RawWorkingState:   []byte(state),
		Result:            result,
		ReconcileAfterSec: reconcileAfter,
	}

	if resp != nil {
		*resp = newHandlerResponse
	} else {
		log.Warn(ctx, "Handler response is nil, cannot set the response", slog.String("State", state))
	}

	return err
}

func (o *TenantOperator) injectSystemUser(
	next orbital.Handler,
) orbital.Handler {
	return func(ctx context.Context, request orbital.HandlerRequest, response *orbital.HandlerResponse) error {
		ctx = cmkcontext.InjectSystemUser(ctx)
		return next(ctx, request, response)
	}
}

// registerHandlers registers all task handlers with the orbital operator
func (o *TenantOperator) registerHandlers(operator *orbital.Operator) error {
	handlers := map[string]orbital.Handler{
		tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():  o.handleCreateTenant,
		tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():      o.handleBlockTenant,
		tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():    o.handleUnblockTenant,
		tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():  o.handleTerminateTenant,
		authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String(): o.handleApplyTenantAuth,
	}

	for action, handler := range handlers {
		handler = o.injectSystemUser(handler)
		err := operator.RegisterHandler(action, handler)
		if err != nil {
			return oops.In(operatorComponent).
				Wrapf(err, "%s: %s", msgRegisteringHandler, action)
		}
	}

	return nil
}

// sendTenantUserGroupsToRegistry sends the user groups of a tenant to the Registry service
func (o *TenantOperator) sendTenantUserGroupsToRegistry(ctx context.Context, tenantID string) (bool, error) {
	//nolint:godox
	// todo: fetch groups from database instead of building them
	groupIAMIDs := []string{
		model.NewIAMIdentifier(constants.TenantAdminGroup, tenantID),
		model.NewIAMIdentifier(constants.TenantAuditorGroup, tenantID),
	}

	groups := make([]*model.Group, len(groupIAMIDs))
	for i, groupIAMID := range groupIAMIDs {
		groups[i] = &model.Group{IAMIdentifier: groupIAMID}
	}

	ctx = model.LogInjectGroups(ctx, groups)

	req := &tenantgrpc.SetTenantUserGroupsRequest{
		Id:         tenantID,
		UserGroups: groupIAMIDs,
	}

	resp, err := o.clientsFactory.Registry().Tenant().SetTenantUserGroups(ctx, req)
	if err != nil {
		log.Error(ctx, "SetTenantUserGroups request failed", err)
		return false, err
	}

	log.Debug(ctx, "Sent user groups to registry", slog.Bool("success", resp.GetSuccess()))

	return resp.GetSuccess(), err
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
		ID:        tenantProto.GetId(),
		Region:    tenantProto.GetRegion(),
		Status:    model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
		OwnerType: tenantProto.GetOwnerType(),
		OwnerID:   tenantProto.GetOwnerId(),
		Role:      model.TenantRole(tenantProto.GetRole().String()),
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
	resp *orbital.HandlerResponse,
) error {
	success, err := o.sendTenantUserGroupsToRegistry(ctx, tenantID)
	if err != nil || !success {
		return newHandlerResponse(ctx, WorkingStateSendingGroupsFailed, orbital.ResultProcessing, resp, err)
	}

	log.Info(ctx, WorkingStateTenantCreatedSuccessfully)

	return newHandlerResponse(ctx, WorkingStateTenantCreatedSuccessfully, orbital.ResultDone, resp, nil)
}

// createTenantSchema creates the tenant schema in the database
func (o *TenantOperator) createTenantSchema(ctx context.Context, tenant *model.Tenant) error {
	err := o.tm.CreateTenant(ctx, tenant)
	if err != nil {
		if errors.Is(err, manager.ErrOnboardingInProgress) {
			log.Info(ctx, "Onboarding in progress, returning early")
			return nil
		}
	}

	return err
}

// createTenantGroups creates the tenant groups
func (o *TenantOperator) createTenantGroups(ctx context.Context, tenant *model.Tenant) error {
	groupCtx := cmkcontext.CreateTenantContext(ctx, tenant.ID)

	err := o.gm.CreateDefaultGroups(groupCtx)
	if err != nil {
		if errors.Is(err, manager.ErrOnboardingInProgress) {
			log.Info(ctx, "Onboarding in progress, returning early")
			return nil
		}
	}

	return err
}

// OIDCConfig extracted from auth properties
type OIDCConfig struct {
	Issuer               string
	JwksURI              string
	Audiences            []string
	AdditionalProperties map[string]string
}

const (
	keyIssuer    = "issuer"
	keyJWKSURI   = "jwks_uri"
	keyAudiences = "audiences"
)

// extractOIDCConfig extracts and validates OIDC configuration from properties map
func extractOIDCConfig(properties map[string]string) (OIDCConfig, error) {
	if properties == nil {
		return OIDCConfig{}, ErrMissingProperties
	}

	var issuer, jwksURI, audiences string

	additionalProperties := make(map[string]string, len(properties))

	for k, v := range properties {
		switch k {
		case keyIssuer:
			issuer = v
		case keyJWKSURI:
			jwksURI = v
		case keyAudiences:
			audiences = v
		default:
			additionalProperties[k] = v
		}
	}

	// Extract issuer (required)
	if issuer == "" {
		return OIDCConfig{}, ErrMissingIssuer
	}

	// Extract optional properties
	cfg := OIDCConfig{
		Issuer:               issuer,
		JwksURI:              jwksURI,
		Audiences:            parseCommaSeparatedValues(audiences),
		AdditionalProperties: additionalProperties,
	}

	return cfg, nil
}

// parseCommaSeparatedValues parses a comma-separated string into a slice of trimmed non-empty strings
// Returns an empty slice if the input is empty or contains no valid values
func parseCommaSeparatedValues(value string) []string {
	if value == "" {
		return []string{}
	}

	var result []string

	for v := range strings.SplitSeq(value, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	// Ensure we always return an empty slice, never nil
	if len(result) == 0 {
		return []string{}
	}

	return result
}
