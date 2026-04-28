package testplugins

import (
	"context"

	"github.com/google/uuid"
	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/common"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keystoremanagement"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

type TestKeystoreManagement struct{}

var _ keystoremanagement.KeystoreManagement = (*TestKeystoreManagement)(nil)

func NewTestKeystoreManagement() *TestKeystoreManagement {
	return &TestKeystoreManagement{}
}

func (s *TestKeystoreManagement) ServiceInfo() api.Info {
	return testInfo{
		configuredType: servicewrapper.KeystoreManagementType,
	}
}

func (s *TestKeystoreManagement) CreateKeystore(
	_ context.Context,
	_ *keystoremanagement.CreateKeystoreRequest,
) (*keystoremanagement.CreateKeystoreResponse, error) {
	return &keystoremanagement.CreateKeystoreResponse{
		Config: common.KeystoreConfig{
			Values: map[string]any{
				"locality":   "test-uuid",
				"commonName": "default.kms.test",
				"managementAccessData": map[string]any{
					"accountId": "mock-account",
					"userId":    "mock-user",
					"random":    uuid.NewString(),
				},
			},
		},
	}, nil
}

func (s *TestKeystoreManagement) DeleteKeystore(
	_ context.Context,
	_ *keystoremanagement.DeleteKeystoreRequest,
) (*keystoremanagement.DeleteKeystoreResponse, error) {
	return &keystoremanagement.DeleteKeystoreResponse{}, nil
}
