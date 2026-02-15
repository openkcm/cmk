package cmkplugincatalog_test

import (
	"testing"

	"github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	keystoreopv1 "github.com/openkcm/plugin-sdk/proto/plugin/keystore/operations/v1"

	"github.com/openkcm/cmk/internal/config"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     &config.Config{},
			wantErr: false,
		},
		{
			name: "plugin disabled",
			cfg: &config.Config{
				Plugins: []catalog.PluginConfig{
					{
						Name:     "TestPlugin",
						Type:     keystoreopv1.Type,
						Checksum: "abc123",
						Path:     "./plugin",
						Args:     []string{"--debug"},
						LogLevel: "debug",
						Disabled: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid plugin path",
			cfg: &config.Config{
				Plugins: []catalog.PluginConfig{
					{
						Name:     "InvalidPlugin",
						Type:     keystoreopv1.Type,
						Checksum: "xyz789",
						Path:     "./invalid_path",
						Args:     []string{"--debug"},
						LogLevel: "debug",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			ctlg, err := cmkplugincatalog.New(ctx, tc.cfg)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, ctlg)
				assert.IsType(t, &cmkplugincatalog.Registry{}, ctlg)
			}
		})
	}
}
