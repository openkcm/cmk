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

// CreationStatus is the keystore creation state reported by a provider plugin.
// An empty status means the plugin did not set the field.
type CreationStatus string

const (
	CreationStatusActive            CreationStatus = "ACTIVE"
	CreationStatusPendingActivation CreationStatus = "PENDING_ACTIVATION"
	CreationStatusFailed            CreationStatus = "FAILED"
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
	GetKeystoreStatus(ctx context.Context, req *GetKeystoreStatusRequest) (*GetKeystoreStatusResponse, error)
	FinalizeKeystoreSetup(ctx context.Context, req *FinalizeKeystoreSetupRequest) (*FinalizeKeystoreSetupResponse, error)
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
	Status               CreationStatus
	AccountID            string
	ErrorMessage         string
}

func (c *CreateKeystoreResponse) ToKeystoreConfig() common.KeystoreConfig {
	return keystoreConfigFromManagement(c.RoleManagementConfig, c.KeyManagementConfig, c.SupportedRegions)
}

func keystoreConfigFromManagement(
	role ManagementConfig,
	key ManagementConfig,
	regions []config.Region,
) common.KeystoreConfig {
	return common.KeystoreConfig{
		Values: map[string]any{
			"roleManagementConfig": map[string]any{
				"localityID": role.LocalityID,
				"commonName": role.CommonName,
				"accessData": role.AccessData.Values,
			},
			"keyManagementConfig": map[string]any{
				"localityID": key.LocalityID,
				"commonName": key.CommonName,
				"accessData": key.AccessData.Values,
			},
			"supportedRegions": regions,
		},
	}
}

type GetKeystoreStatusRequest struct {
	AccountID string
}

type GetKeystoreStatusResponse struct {
	Status       CreationStatus
	ErrorMessage string
}

type FinalizeKeystoreSetupRequest struct {
	AccountID string
	Values    map[string]any
}

type FinalizeKeystoreSetupResponse struct {
	RoleManagementConfig ManagementConfig
	KeyManagementConfig  ManagementConfig
	SupportedRegions     []config.Region
	Status               CreationStatus
	ErrorMessage         string
}

func (c *FinalizeKeystoreSetupResponse) ToKeystoreConfig() common.KeystoreConfig {
	return keystoreConfigFromManagement(c.RoleManagementConfig, c.KeyManagementConfig, c.SupportedRegions)
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
