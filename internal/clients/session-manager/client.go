package sessionmanager

import (
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"

	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"
)

const (
	DefaultThrottleInterval = 5 * time.Second
)

type Client struct {
	oidc oidcmappinggrpc.ServiceClient

	conn *commongrpc.DynamicClientConn
}

func NewClient(cfg *commoncfg.GRPCClient) (*Client, error) {
	conn, err := commongrpc.NewDynamicClientConn(cfg, DefaultThrottleInterval)
	if err != nil {
		return nil, err
	}

	return &Client{
		oidc: oidcmappinggrpc.NewServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *Client) OIDCMapping() oidcmappinggrpc.ServiceClient {
	return c.oidc
}

func (c *Client) Close() error {
	return c.conn.Close()
}
