package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/openkcm/plugin-sdk/pkg/plugin"
	"google.golang.org/protobuf/types/known/structpb"

	kscommonv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/common/v1"
	keymanv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/management/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

// TestPlugin is a simple test implementation of KeystoreProviderClient
type TestPlugin struct {
	keymanv1.UnsafeKeystoreProviderServer
	configv1.UnsafeConfigServer

	logger hclog.Logger
}

func (p *TestPlugin) CreateKeystore(
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

func (p *TestPlugin) DeleteKeystore(
	_ context.Context,
	_ *keymanv1.DeleteKeystoreRequest,
) (*keymanv1.DeleteKeystoreResponse, error) {
	return &keymanv1.DeleteKeystoreResponse{}, nil
}

func New() *TestPlugin {
	return &TestPlugin{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "keystoreop-test-plugin",
			Level: hclog.LevelFromString("DEBUG"),
		}),
	}
}

// Configure configures the plugin.

func (p *TestPlugin) Configure(
	_ context.Context,
	_ *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	p.logger.Info("Configure method has been called;")

	var buildInfo = "{}"

	return &configv1.ConfigureResponse{
		BuildInfo: &buildInfo,
	}, nil
}

func main() {
	p := New()

	plugin.Serve(
		keymanv1.KeystoreProviderPluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}
