package daemon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestServer(t *testing.T) {
	dbConf := testutils.NewIsolatedDB(t, testutils.TestDB)

	cfg := &config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8081",
		},
		Database: dbConf,
	}

	t.Run("Should create CMK Server", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), cfg)
		assert.NoError(t, err)
		assert.NotNil(t, s)

		err = s.Start(t.Context())
		assert.NoError(t, err)
		err = s.Close(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should fail on create CMK Server without database", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), &config.Config{})
		assert.Error(t, err)
		assert.Nil(t, s)
	})
}
