package main

import (
	"context"

	"github.com/openkcm/plugin-sdk/pkg/plugin"

	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type TestPlugin struct {
	configv1.UnsafeConfigServer
	systeminformationv1.UnimplementedSystemInformationServiceServer
}

// check if the TestPlugin implements the systeminformationv1.SystemInformationServiceServer interface
var _ systeminformationv1.SystemInformationServiceServer = (*TestPlugin)(nil)

func (p *TestPlugin) Configure(_ context.Context, _ *configv1.ConfigureRequest) (*configv1.ConfigureResponse, error) {
	var buildInfo = "{}"

	return &configv1.ConfigureResponse{
		BuildInfo: &buildInfo,
	}, nil
}

func (p *TestPlugin) Get(_ context.Context, _ *systeminformationv1.GetRequest) (
	*systeminformationv1.GetResponse, error,
) {
	return &systeminformationv1.GetResponse{}, nil
}

func New() *TestPlugin {
	return &TestPlugin{}
}

func main() {
	server := New()

	plugin.Serve(systeminformationv1.SystemInformationServicePluginServer(server),
		configv1.ConfigServiceServer(server))
}
