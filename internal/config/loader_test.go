package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.tools.sap/kms/cmk/internal/config"
)

func buildCfg(t *testing.T) []byte {
	t.Helper()

	contextModels := &config.ContextModels{
		System: config.System{
			Identifier: config.SystemProperty{
				DisplayName: "GTID",
				Internal:    true,
			},
			Region: config.SystemProperty{
				DisplayName: "Region",
				Internal:    true,
			},
			Type: config.SystemProperty{
				DisplayName: "Type",
				Internal:    true,
			},
			OptionalProperties: map[string]config.SystemProperty{},
		},
	}

	bytes, err := yaml.Marshal(contextModels)
	require.NoError(t, err)

	res, err := yaml.Marshal(&config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8082",
		},

		BaseConfig: commoncfg.BaseConfig{
			Logger: commoncfg.Logger{
				Format: "json",
				Level:  "info",
			},
		},
		ConfigurableContext: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  string(bytes),
		},
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
	})
	require.NoError(t, err)

	return res
}

func TestLoadConfig(t *testing.T) {
	t.Run("Should load config", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "config.yaml")

		c := buildCfg(t)

		err := os.WriteFile(file, c, 0o600)
		assert.NoError(t, err)

		_, err = config.LoadConfig(
			commoncfg.WithPaths(dir),
		)
		assert.NoError(t, err)
	})

	t.Run("Should fail on missing configurableContext", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "config.yaml")

		c, err := yaml.Marshal(&config.Config{})
		assert.NoError(t, err)

		err = os.WriteFile(file, c, 0o600)
		assert.NoError(t, err)

		_, err = config.LoadConfig(
			commoncfg.WithPaths(dir),
		)
		assert.Error(t, err)
	})
}
