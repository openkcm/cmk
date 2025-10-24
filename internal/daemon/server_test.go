package daemon_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/daemon"
)

var conf = &config.Config{
	HTTP: config.HTTPServer{
		Address: "localhost:8081",
	},
	Database: config.Database{
		Host: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "localhost",
		},
		User: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "postgres",
		},
		Secret: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "secret",
		},
		Name: "cmk",
		Port: "5433",
	},
}

func TestNewServer(t *testing.T) {
	t.Run("Should create CMK Server", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), conf)
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("Should fail on create CMK Server without database", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), &config.Config{})
		assert.Error(t, err)
		assert.Nil(t, s)
	})
}

func TestStartAndStop(t *testing.T) {
	t.Run("Start server", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), conf)
		assert.NoError(t, err)
		err = s.Start(t.Context())
		assert.NoError(t, err)
		err = s.Close(t.Context())
		assert.NoError(t, err)
	})
}
