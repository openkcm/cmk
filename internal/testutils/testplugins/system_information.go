package testplugins

import (
	"context"
	"log/slog"

	"github.com/openkcm/plugin-sdk/pkg/catalog"

	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"
	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"
)

type SystemInformation struct {
	systeminformationv1.UnsafeSystemInformationServiceServer
	configv1.UnsafeConfigServer
}

func NewSystemInformation() catalog.BuiltInPlugin {
	p := &SystemInformation{}
	return catalog.MakeBuiltIn(
		Name,
		systeminformationv1.SystemInformationServicePluginServer(p),
		configv1.ConfigServiceServer(p),
	)
}

func (p *SystemInformation) Configure(
	_ context.Context,
	req *configv1.ConfigureRequest,
) (*configv1.ConfigureResponse, error) {
	slog.Info("Configuring plugin")

	return &configv1.ConfigureResponse{}, nil
}

func (p *SystemInformation) Get(
	_ context.Context,
	_ *systeminformationv1.GetRequest,
) (
	*systeminformationv1.GetResponse, error,
) {
	return &systeminformationv1.GetResponse{}, nil
}
