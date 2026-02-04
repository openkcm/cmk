package registry

import (
	"io"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	mappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients/registry/systems"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Service interface {
	io.Closer

	System() systems.ServiceClient
	Tenant() tenantgrpc.ServiceClient
	Mapping() mappinggrpc.ServiceClient
}

type service struct {
	system  systems.ServiceClient
	tenant  tenantgrpc.ServiceClient
	mapping mappinggrpc.ServiceClient

	grpcConn *commongrpc.DynamicClientConn
}

func NewService(rg *commoncfg.GRPCClient) (Service, error) {
	conn, err := commongrpc.NewDynamicClientConn(rg, DefaultThrottleInterval)
	if err != nil {
		return nil, err
	}

	sysClient, err := systems.NewSystemsClient(conn)
	if err != nil {
		return nil, err
	}

	tenantClient := tenantgrpc.NewServiceClient(conn)

	mappingClient := mappinggrpc.NewServiceClient(conn)

	return &service{
		system:   sysClient,
		tenant:   tenantClient,
		mapping:  mappingClient,
		grpcConn: conn,
	}, nil
}

func (rs *service) System() systems.ServiceClient {
	return rs.system
}

func (rs *service) Tenant() tenantgrpc.ServiceClient {
	return rs.tenant
}

func (rs *service) Mapping() mappinggrpc.ServiceClient {
	return rs.mapping
}

func (rs *service) Close() error {
	return rs.grpcConn.Close()
}
