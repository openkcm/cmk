package testplugins

import (
	"context"

	"github.com/openkcm/plugin-sdk/api"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/systeminformation"
	servicewrapper "github.com/openkcm/cmk/internal/pluginregistry/service/wrapper"
)

type TestSystemInformation struct{}

var _ systeminformation.SystemInformation = (*TestSystemInformation)(nil)

func NewTestSystemInformation() *TestSystemInformation {
	return &TestSystemInformation{}
}

func (s *TestSystemInformation) ServiceInfo() api.Info {
	return testInfo{
		configuredType: servicewrapper.SystemInformationServiceType,
	}
}

func (s *TestSystemInformation) GetSystemInfo(
	_ context.Context,
	_ *systeminformation.GetSystemInfoRequest,
) (*systeminformation.GetSystemInfoResponse, error) {
	return &systeminformation.GetSystemInfoResponse{}, nil
}
