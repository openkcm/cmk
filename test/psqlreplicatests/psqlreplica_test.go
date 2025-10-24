package psqlreplicatests_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/db"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	integrationutils "github.com/openkcm/cmk-core/test/integration_utils"
)

func TestConnectToPsqlWithReplica(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name: "No replica config",
			cfg: config.Config{
				Database: integrationutils.DB,
			},
			wantErr: false,
		},
		{
			name: "Replica config",
			cfg: config.Config{
				Database:         integrationutils.DB,
				DatabaseReplicas: integrationutils.ReplicaDB,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbConn, err := db.StartDBConnection(
				tc.cfg.Database,
				tc.cfg.DatabaseReplicas,
			)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NotNil(t, dbConn)
				require.NoError(t, err)
			}
		})
	}
}

func TestQueryToPsqlWithReplica(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name: "Use replica to read from database",
			cfg: config.Config{
				Database:         integrationutils.DB,
				DatabaseReplicas: integrationutils.ReplicaDB,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
				Models: []driver.TenantTabler{&model.System{}},
			}, testutils.WithDatabase(integrationutils.DB))

			ctx := testutils.CreateCtxWithTenant(tenants[0])

			repository := sql.NewRepository(db)

			sys := testutils.NewSystem(func(_ *model.System) {})
			testutils.CreateTestEntities(ctx, t, repository, sys)

			found, err := repository.First(
				ctx,
				sys,
				repo.Query{},
			)
			assert.NoError(t, err)
			assert.True(t, found)
		})
	}
}
