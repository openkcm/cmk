package registry

import (
	"io"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	mappinggrpcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	tenantgrpcv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/cmk/internal/clients/registry/systems"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Service interface {
	io.Closer

	System() systems.ServiceClient
	Tenant() tenantgrpcv1.ServiceClient
	Mapping() mappinggrpcv1.ServiceClient
}

type service struct {
	system  systems.ServiceClient
	tenant  tenantgrpcv1.ServiceClient
	mapping mappinggrpcv1.ServiceClient

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

	tenantClient := tenantgrpcv1.NewServiceClient(conn)

	mappingClient := mappinggrpcv1.NewServiceClient(conn)

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

func (rs *service) Tenant() tenantgrpcv1.ServiceClient {
	return rs.tenant
}

func (rs *service) Mapping() mappinggrpcv1.ServiceClient {
	return rs.mapping
}

func (rs *service) Close() error {
	return rs.grpcConn.Close()
}
