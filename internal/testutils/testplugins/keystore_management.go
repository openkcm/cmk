package testplugins

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"google.golang.org/protobuf/types/known/structpb"

	kscommonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	keymanv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type KeystoreManagement struct {
	keymanv1.UnsafeKeystoreProviderServer
	configv1.UnsafeConfigServer

	logger hclog.Logger
}

func NewKeystoreManagement() catalog.BuiltInPlugin {
	p := &KeystoreManagement{}
	return catalog.MakeBuiltIn(
		Name,
		keymanv1.KeystoreProviderPluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

func (p *KeystoreManagement) SetLogger(logger hclog.Logger) {
	p.logger = logger
	p.logger.Info("SetLogger method has been called;")
}

func (p *KeystoreManagement) CreateKeystore(
	_ context.Context,
	_ *keymanv1.CreateKeystoreRequest,
) (*keymanv1.CreateKeystoreResponse, error) {
	p.logger.Info("CreateKeystore method has been called;")

	config := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"locality":   structpb.NewStringValue("test-uuid"),
			"commonName": structpb.NewStringValue("default.kms.test"),
			"managementAccessData": structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"accountId": structpb.NewStringValue("mock-account"),
					"userId":    structpb.NewStringValue("mock-user"),
					"random":    structpb.NewStringValue(uuid.NewString()),
				},
			}),
		},
	}

	return &keymanv1.CreateKeystoreResponse{
		Config: &kscommonv1.KeystoreInstanceConfig{
			Values: config,
		},
	}, nil
}

func (p *KeystoreManagement) DeleteKeystore(
	_ context.Context,
	_ *keymanv1.DeleteKeystoreRequest,
) (*keymanv1.DeleteKeystoreResponse, error) {
	return &keymanv1.DeleteKeystoreResponse{}, nil
}

// Configure configures the plugin.

func (p *KeystoreManagement) Configure(
	_ context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	p.logger.Info("Configure method has been called;")

	buildInfo := "{}"

	return &configv1.ConfigureResponse{
		BuildInfo: &buildInfo,
	}, nil
}
