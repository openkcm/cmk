package daemon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/daemon"
	"github.tools.sap/kms/cmk/internal/db"
	"github.tools.sap/kms/cmk/internal/testutils"
)

func TestServer(t *testing.T) {
	dbConf := testutils.NewIsolatedDB(t, testutils.TestDB)

	cfg := &config.Config{
		HTTP: config.HTTPServer{
			Address: "localhost:8081",
		},
		Database: dbConf,
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
