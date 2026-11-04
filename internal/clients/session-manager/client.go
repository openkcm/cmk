package sessionmanager

import (
	"io"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	trustmappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/trustmapping/v1"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Service interface {
	io.Closer

	TrustMapping() trustmappingv1.ServiceClient
}

type service struct {
	trust trustmappingv1.ServiceClient

	conn *commongrpc.DynamicClientConn
}

func NewService(cfg *commoncfg.GRPCClient) (Service, error) {
	conn, err := commongrpc.NewDynamicClientConn(cfg, DefaultThrottleInterval)
	if err != nil {
		return nil, err
	}

	return &service{
		trust: trustmappingv1.NewServiceClient(conn),
		conn:  conn,
	}, nil
}

func (c *service) TrustMapping() trustmappingv1.ServiceClient {
	return c.trust
}

func (c *service) Close() error {
	return c.conn.Close()
}
