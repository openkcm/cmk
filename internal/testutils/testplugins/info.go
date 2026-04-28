package testplugins

import (
	"github.com/openkcm/plugin-sdk/api"
)

// testInfo implements api.Info for test service implementations.
type testInfo struct {
	configuredTags []string
	configuredType string
}

func (testInfo) Name() string     { return Name }
func (t testInfo) Type() string   { return t.configuredType }
func (t testInfo) Tags() []string { return t.configuredTags }
func (testInfo) Build() string    { return "{}" }
func (testInfo) Version() uint    { return 1 }

var _ api.Info = testInfo{}
