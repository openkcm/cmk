package scim

import (
	"log/slog"
	"testing"

	"github.com/magodo/slog2hclog"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/plugin-sdk/pkg/hclog2slog"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
)

func getLogger() *slog.Logger {
	logLevelPlugin := new(slog.LevelVar)
	logLevelPlugin.Set(slog.LevelError)
	return hclog2slog.New(slog2hclog.New(slog.Default(), logLevelPlugin))
}

func (p *Plugin) SetTestClient(t *testing.T, host string, groupFilterAttribute, userFilterAttribute string) {
	t.Helper()

	secretRef := commoncfg.SecretRef{
		Type: commoncfg.BasicSecretType,
		Basic: commoncfg.BasicAuth{
			Username: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			Password: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
		},
	}

	c, err := client.NewClient(secretRef, getLogger())
	assert.NoError(t, err)

	p.scimClient = c
	p.params = &Params{
		BaseHost:                host,
		GroupAttribute:          groupFilterAttribute,
		UserAttribute:           userFilterAttribute,
		AllowSearchUsersByGroup: true,
	}
}
