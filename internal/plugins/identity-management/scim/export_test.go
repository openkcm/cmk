package scim

import (
	"log/slog"
	"os"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
)

func getLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func (p *Plugin) SetTestClient(t *testing.T, host string, groupFilterAttribute, userFilterAttribute, groupMembersAttribute string, allowSearchUsersByGroup bool) {
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
		GroupMembersAttribute:   groupMembersAttribute,
		AllowSearchUsersByGroup: allowSearchUsersByGroup,
	}
}
