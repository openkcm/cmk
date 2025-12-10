package systems

import (
	"context"
	"errors"
	"time"

	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	retry "github.com/avast/retry-go/v5"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"

	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
)

const (
	defaultListSystemsLimit = 100
	delay                   = 100 * time.Millisecond
	maxDelay                = 1 * time.Second
	attempts                = 3
)

var (
	ErrGettingSystem       = errors.New("failed to get system")
	ErrClientInternalError = errors.New("internal error in systems client")

	ErrKeyClaimAlreadyActive     = status.Error(codes.FailedPrecondition, "key claim is already active")
	ErrKeyClaimAlreadyInactive   = status.Error(codes.FailedPrecondition, "key claim is already inactive")
	ErrSystemNotFound            = status.Error(codes.NotFound, "system not found")
	ErrSystemIsNotLinkedToTenant = status.Error(codes.FailedPrecondition, "system is not linked to the tenant")
)

// SystemFilter is used for filtering systems results using defined fields.
type SystemFilter struct {
	// Region of a system. Forms CMK composite key with the ExternalID
	Region string
	// Externally (to plugin) provided ID of a system.
	ExternalID string
	// TenantID is the ID of the CMK tenant to which the system is linked in registry. Typically current tenant ID.
	TenantID string
	// Limit defines systems count to be returned.
	Limit int32
}

type Config struct {
	Delay    time.Duration
	MaxDelay time.Duration
	Attempts uint
}

type ServiceClient interface {
	systemgrpc.ServiceClient

	GetSystemsWithFilter(ctx context.Context, filter SystemFilter) ([]*model.System, error)
	ExtendedUpdateSystemL1KeyClaim(ctx context.Context, filter SystemFilter, l1KeyClaim bool) error
}

// Client of registry service systems API.
type client struct {
	systemgrpc.ServiceClient

	config Config
}

var _ ServiceClient = (*client)(nil)

type Option func(*client)

// NewSystemsClient creates new instance of Client.
func NewSystemsClient(grpcClient *commongrpc.DynamicClientConn, options ...Option) (ServiceClient, error) {
	if grpcClient == nil {
		return nil, ErrSystemsClientDoesNotExist
	}

	client := &client{
		ServiceClient: systemgrpc.NewServiceClient(grpcClient),
		config: Config{
			Delay:    delay,
			Attempts: attempts,
			MaxDelay: maxDelay,
		},
	}

	for _, o := range options {
		o(client)
	}

	return client, nil
}

// GetSystemsWithFilter using systems client.
func (c *client) GetSystemsWithFilter(ctx context.Context,
	filter SystemFilter,
) ([]*model.System, error) {
	var (
		grpcSystems []*systemgrpc.System
		err         error
	)

	retrier := c.getRetrier()

	err = retrier.Do(
		func() error {
			var limit int32 = defaultListSystemsLimit

			if filter.Limit != 0 {
				// We do not page for filter.Limit == 0
				limit = filter.Limit
			}

			pageToken := ""

			for {
				resp, err := c.ListSystems(ctx, &systemgrpc.ListSystemsRequest{
					Region:     filter.Region,
					ExternalId: filter.ExternalID,
					TenantId:   filter.TenantID,
					Limit:      limit,
					PageToken:  pageToken,
				})
				if status.Code(err) == codes.Internal {
					return errs.Wrap(ErrClientInternalError, err)
				} else if err != nil {
					return errs.Wrap(ErrSystemsClientFailedGettingSystems, err)
				}

				newSystems := resp.GetSystems()
				pageToken = resp.GetNextPageToken()

				grpcSystems = append(grpcSystems, newSystems...)

				// We do not page for filter.Limit == 0
				if pageToken == "" || filter.Limit == 0 {
					break
				}
			}

			return nil
		},
	)
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystem, err)
	}

	systems, err := MapRegistrySystemsToCmkSystems(grpcSystems)
	if err != nil {
		return nil, errs.Wrap(ErrGettingSystem, err)
	}

	return systems, nil
}

// ExtendedUpdateSystemL1KeyClaim using systems client.
func (c *client) ExtendedUpdateSystemL1KeyClaim(
	ctx context.Context,
	filter SystemFilter,
	l1KeyClaim bool,
) error {
	var (
		resp *systemgrpc.UpdateSystemL1KeyClaimResponse
		err  error
	)

	retrier := c.getRetrier()

	err = retrier.Do(
		func() error {
			resp, err = c.UpdateSystemL1KeyClaim(ctx,
				&systemgrpc.UpdateSystemL1KeyClaimRequest{
					Region:     filter.Region,
					ExternalId: filter.ExternalID,
					TenantId:   filter.TenantID,
					L1KeyClaim: l1KeyClaim,
				})
			if status.Code(err) == codes.Internal {
				return errs.Wrap(ErrClientInternalError, err)
			} else if err != nil {
				return errs.Wrap(ErrSystemsClientFailedUpdatingKeyClaim, err)
			}

			return nil
		},
	)
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return ErrSystemsServerFailedUpdatingKeyClaim
	}

	return nil
}

func (c *client) getRetrier() *retry.Retrier {
	return retry.New(
		retry.RetryIf(func(err error) bool {
			return status.Code(err) == codes.Unavailable
		}),
		retry.Delay(c.config.Delay),
		retry.MaxDelay(c.config.MaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.Attempts(c.config.Attempts),
		retry.LastErrorOnly(true),
	)
}
