package psqlreplicatests_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

type PSQLReplicaSuite struct {
	suite.Suite

	postgresPort string
	dbConfig     config.Database
	replicDB     []config.Database
}

func (s *PSQLReplicaSuite) SetupSuite() {
	cfg := &config.Config{
		Database: integrationutils.DB,
	}
	testutils.StartPostgresSQL(s.T(), &cfg.Database)

	s.dbConfig = cfg.Database
	s.replicDB = integrationutils.ReplicaDB
	s.replicDB[0].Port = cfg.Database.Port
	s.postgresPort = cfg.Database.Port
}

func (s *PSQLReplicaSuite) TestConnectToPsqlWithoutReplica() {
	cfg := config.Config{
		Database: s.dbConfig,
	}

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)

	s.Require().NotNil(dbConn)
	s.Require().NoError(err)
}

func (s *PSQLReplicaSuite) TestConnectToPsqlWithReplica() {
	cfg := config.Config{
		Database:         s.dbConfig,
		DatabaseReplicas: s.replicDB,
	}

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)

	s.Require().NotNil(dbConn)
	s.Require().NoError(err)
}

func (s *PSQLReplicaSuite) TestQueryToPsqlWithReplica() {
	db, tenants, _ := testutils.NewTestDB(s.T(), testutils.TestDBConfig{})

	ctx := testutils.CreateCtxWithTenant(tenants[0])
	repository := sql.NewRepository(db)

	// Create test entity
	sys := testutils.NewSystem(func(_ *model.System) {})
	testutils.CreateTestEntities(ctx, s.T(), repository, sys)

	// Query the entity
	found, err := repository.First(
		ctx,
		sys,
		repo.Query{},
	)

	s.NoError(err)
	s.True(found)
}

func (s *PSQLReplicaSuite) TestDatabaseConnectionError() {
	invalidConfig := s.dbConfig
	invalidConfig.Port = "9999"

	cfg := config.Config{
		Database: invalidConfig,
	}

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)

	s.Nil(dbConn)
	s.Error(err)
}

func (s *PSQLReplicaSuite) TestReplicaConnectionError() {
	invalidReplica := s.replicDB
	invalidReplica[0].Port = "9999"

	cfg := config.Config{
		Database:         s.dbConfig,
		DatabaseReplicas: invalidReplica,
	}

	dbCon, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)
	s.Nil(dbCon)
	s.Error(err)
}

func (s *PSQLReplicaSuite) TestMultipleReplicas() {
	multipleReplicas := []config.Database{
		{
			Host:   s.replicDB[0].Host,
			User:   s.replicDB[0].User,
			Secret: s.replicDB[0].Secret,
			Name:   s.replicDB[0].Name,
			Port:   s.postgresPort,
		},
		{
			Host:   s.replicDB[0].Host,
			User:   s.replicDB[0].User,
			Secret: s.replicDB[0].Secret,
			Name:   s.replicDB[0].Name,
			Port:   s.postgresPort,
		},
	}

	cfg := config.Config{
		Database:         s.dbConfig,
		DatabaseReplicas: multipleReplicas,
	}

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)

	s.Require().NotNil(dbConn)
	s.Require().NoError(err)
}

func (s *PSQLReplicaSuite) TestEmptyReplicaConfig() {
	cfg := config.Config{
		Database:         s.dbConfig,
		DatabaseReplicas: []config.Database{},
	}

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		cfg.Database,
		cfg.DatabaseReplicas,
	)

	s.Require().NotNil(dbConn)
	s.Require().NoError(err)
}

func TestPSQLReplicaSuite(t *testing.T) {
	suite.Run(t, new(PSQLReplicaSuite))
}
