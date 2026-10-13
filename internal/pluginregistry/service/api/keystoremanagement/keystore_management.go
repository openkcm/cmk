package keystoremanagement

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
)

type TrustType string

const (
	TrustTypeManagement TrustType = "MANAGEMENT"
	TrustTypeCrypto     TrustType = "CRYPTO"
)

type ManagementConfig struct {
	// V1 Fields
	LocalityID string
	CommonName string
	AccessData common.KeystoreConfig
}

type KeystoreManagement interface {
	ServiceInfo() api.Info

	CreateKeystore(ctx context.Context, req *CreateKeystoreRequest) (*CreateKeystoreResponse, error)
	DeleteKeystore(ctx context.Context, req *DeleteKeystoreRequest) (*DeleteKeystoreResponse, error)
	GrantTrust(ctx context.Context, req *GrantTrustRequest) (*GrantTrustResponse, error)
	RemoveTrust(ctx context.Context, req *RemoveTrustRequest) (*RemoveTrustResponse, error)
}

type CreateKeystoreRequest struct {
	// V1 Fields
	Values map[string]any
}

type CreateKeystoreResponse struct {
	// V1 Fields
	RoleManagementConfig ManagementConfig
	KeyManagementConfig  ManagementConfig
	SupportedRegions     []config.Region
}

func (c *CreateKeystoreResponse) ToKeystoreConfig() common.KeystoreConfig {
	return common.KeystoreConfig{
		Values: map[string]any{
			"roleManagementConfig": map[string]any{
				"localityID": c.RoleManagementConfig.LocalityID,
				"commonName": c.RoleManagementConfig.CommonName,
				"accessData": c.RoleManagementConfig.AccessData.Values,
			},
			"keyManagementConfig": map[string]any{
				"localityID": c.KeyManagementConfig.LocalityID,
				"commonName": c.KeyManagementConfig.CommonName,
				"accessData": c.KeyManagementConfig.AccessData.Values,
			},
			"supportedRegions": c.SupportedRegions,
		},
	}
}

type DeleteKeystoreRequest struct {
	// V1 Fields
	Config common.KeystoreConfig
}

type DeleteKeystoreResponse struct{}

type GrantTrustRequest struct {
	// V1 Fields
	Config  common.KeystoreConfig
	Subject string
	Region  string
	Type    TrustType
}

type GrantTrustResponse struct {
	AccessData common.KeystoreConfig
}

type RemoveTrustRequest struct {
	// V1 Fields
	Config     common.KeystoreConfig
	AccessData common.KeystoreConfig
}

type RemoveTrustResponse struct{}
