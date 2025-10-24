package registry

import (
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk-core/internal/clients/registry/systems"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Service struct {
	system *systems.Client
	tenant tenantgrpc.ServiceClient

	grpcConn *commongrpc.DynamicClientConn
}

func NewService(rg *commoncfg.GRPCClient) (*Service, error) {
	conn, err := commongrpc.NewDynamicClientConn(rg, DefaultThrottleInterval)
	if err != nil {
		return nil, err
	}

	sysClient, err := systems.NewSystemsClient(conn)
	if err != nil {
		return nil, err
	}

	tenantClient := tenantgrpc.NewServiceClient(conn)

	return &Service{
		system:   sysClient,
		tenant:   tenantClient,
		grpcConn: conn,
	}, nil
}

func (rs *Service) System() *systems.Client {
	return rs.system
}

func (rs *Service) Tenant() tenantgrpc.ServiceClient {
	return rs.tenant
}

func (rs *Service) Close() error {
	return rs.grpcConn.Close()
}
