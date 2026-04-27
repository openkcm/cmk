package dsn_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
)

func TestFromDBConfig(t *testing.T) {
	t.Run("Should have no cert", func(t *testing.T) {
		res, err := dsn.FromDBConfig(config.Database{
			Host: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			User: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			Secret: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			Parameters: config.DBParameters{
				SSL: config.DBSSL{
					Mode: "",
				},
			},
		})
		assert.NoError(t, err)
		assert.Equal(
			t,
			"host= user= password= dbname= port= default_query_exec_mode=simple_protocol sslmode=",
			res,
		)
	})

	t.Run("Should have all cert options", func(t *testing.T) {
		res, err := dsn.FromDBConfig(config.Database{
			Host: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			User: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			Secret: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "",
			},
			Parameters: config.DBParameters{
				SSL: config.DBSSL{
					Mode:     "",
					RootCert: "test",
					Cert:     "test",
					Key:      "test",
				},
			},
		})
		assert.NoError(t, err)
		assert.Equal(
			t,
			"host= user= password= dbname= port= default_query_exec_mode=simple_protocol sslmode= sslrootcert=test sslcert=test sslkey=test",
			res,
		)
	})
}
