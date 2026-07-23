// Package psqlreplicatests verifies db.StartDBConnection's replica handling.
package psqlreplicatests_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/testutils"
)

const replicaContainer = "testcontainers-postgresql-replica-shared"

type PSQLReplicaSuite struct {
	suite.Suite

	primary config.Database
	replica config.Database
}

func (s *PSQLReplicaSuite) SetupSuite() {
	primary := testutils.TestDB
	testutils.StartPostgresSQL(s.T(), &primary)
	s.primary = primary

	replica := testutils.TestDB
	testutils.StartPostgresSQL(s.T(), &replica,
		testcontainers.WithReuseByName(replicaContainer))
	s.replica = replica
}

func (s *PSQLReplicaSuite) TestConnectWithReplica() {
	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.primary,
		[]config.Database{s.replica},
		nil,
	)

	s.Require().NoError(err)
	s.Require().NotNil(dbConn)
}

func (s *PSQLReplicaSuite) TestEmptyReplicaConfig() {
	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.primary,
		[]config.Database{},
		nil,
	)

	s.Require().NoError(err)
	s.Require().NotNil(dbConn)
}

func (s *PSQLReplicaSuite) TestReplicaConnectionError() {
	badReplica := s.replica
	badReplica.Port = "1" // unreachable

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.primary,
		[]config.Database{badReplica},
		nil,
	)

	s.Require().Error(err)
	s.Require().Nil(dbConn)
	s.Require().ErrorIs(err, db.ErrDBResolver)
}

func TestPSQLReplicaSuite(t *testing.T) {
	suite.Run(t, new(PSQLReplicaSuite))
}
