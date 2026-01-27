package daemon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/daemon"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestServer(t *testing.T) {
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

	cfg := &config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8081",
		},
		Database: dbCfg,
	}

	dbCon, err := db.StartDB(t.Context(), cfg)
	assert.NoError(t, err)

	t.Run("Should create CMK Server", func(t *testing.T) {
		s, err := daemon.NewCMKServer(t.Context(), cfg, dbCon)
		assert.NoError(t, err)
		assert.NotNil(t, s)

		err = s.Start(t.Context())
		assert.NoError(t, err)
		err = s.Close(t.Context())
		assert.NoError(t, err)
	})
}
