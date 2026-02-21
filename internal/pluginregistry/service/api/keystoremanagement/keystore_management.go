package keystoremanagement

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
)

type KeystoreManagement interface {
	ServiceInfo() api.Info

	CreateKeystore(ctx context.Context, req *CreateKeystoreRequest) (*CreateKeystoreResponse, error)
	DeleteKeystore(ctx context.Context, req *DeleteKeystoreRequest) (*DeleteKeystoreResponse, error)
}

type CreateKeystoreRequest struct {
	// V1 Fields
	Values map[string]any
}

type CreateKeystoreResponse struct {
	// V1 Fields
	Config common.KeystoreConfig
}

type DeleteKeystoreRequest struct {
	// V1 Fields
	Config common.KeystoreConfig
}

type DeleteKeystoreResponse struct{}
