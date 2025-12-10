package sessionmanager

import (
	"io"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Service interface {
	io.Closer

	OIDCMapping() oidcmappinggrpc.ServiceClient
}

type service struct {
	oidc oidcmappinggrpc.ServiceClient

	conn *commongrpc.DynamicClientConn
}

func NewService(cfg *commoncfg.GRPCClient) (Service, error) {
	conn, err := commongrpc.NewDynamicClientConn(cfg, DefaultThrottleInterval)
	if err != nil {
		return nil, err
	}

	return &service{
		oidc: oidcmappinggrpc.NewServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *service) OIDCMapping() oidcmappinggrpc.ServiceClient {
	return c.oidc
}

func (c *service) Close() error {
	return c.conn.Close()
}
