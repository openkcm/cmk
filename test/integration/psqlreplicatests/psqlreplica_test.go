// Package psqlreplicatests verifies CMK's db layer against a real PostgreSQL
// primary + streaming standby via testutils.StartPostgresWithReplica.
//
// CMK's repository reads go through multitenancy.WithTenant, which opens a
// transaction and pins to the primary; the transaction-pinning tests below
// document why replicas are never reached today.
package psqlreplicatests_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db"
	"github.com/openkcm/cmk/internal/testutils"
)

// walCatchupTimeout bounds how long we wait for the standby to apply a
// transaction written on the primary.
const walCatchupTimeout = 5 * time.Second

// pgReadOnlyTxn is the SQLSTATE returned when a write hits a hot-standby.
const pgReadOnlyTxn = "25006"

type PSQLReplicaSuite struct {
	suite.Suite

	pair testutils.ReplicaPair
}

func (s *PSQLReplicaSuite) SetupSuite() {
	s.pair = testutils.StartPostgresWithReplica(s.T())
}

func (s *PSQLReplicaSuite) openConn() *multitenancy.DB {
	s.T().Helper()
	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.pair.Primary,
		[]config.Database{s.pair.Replica},
		nil,
	)
	s.Require().NoError(err)
	s.Require().NotNil(dbConn)
	return dbConn
}

// onReplica returns pg_is_in_recovery() on whichever pool dbresolver routed
// the supplied gorm.DB to. true = standby, false = primary.
func onReplica(t *testing.T, tx *gorm.DB) bool {
	t.Helper()
	var inRecovery bool
	if err := tx.Raw(`SELECT pg_is_in_recovery()`).Scan(&inRecovery).Error; err != nil {
		t.Fatalf("pg_is_in_recovery: %v", err)
	}
	return inRecovery
}

// TestConnectWithReplica verifies db.StartDBConnection succeeds with one real
// streaming replica configured.
func (s *PSQLReplicaSuite) TestConnectWithReplica() {
	s.Require().NotNil(s.openConn())
}

// TestStreamingReplicationIsLive asserts the standby is actually following
// the primary rather than sitting in recovery mode on a frozen snapshot.
func (s *PSQLReplicaSuite) TestStreamingReplicationIsLive() {
	dbConn := s.openConn()

	s.True(onReplica(s.T(), dbConn.Clauses(dbresolver.Read).Session(&gorm.Session{})),
		"standby must be in recovery mode")

	var streamingSenders int
	s.Require().NoError(
		dbConn.Clauses(dbresolver.Write).
			Raw(`SELECT count(*) FROM pg_stat_replication WHERE state = 'streaming'`).
			Scan(&streamingSenders).Error,
	)
	s.GreaterOrEqual(streamingSenders, 1, "primary must show a live streaming sender")
}

// TestExplicitReadClauseRoutesToReplica is the dbresolver contract test:
// Clauses(Read) lands on the standby, Clauses(Write) lands on the primary.
func (s *PSQLReplicaSuite) TestExplicitReadClauseRoutesToReplica() {
	dbConn := s.openConn()

	const createSQL = `CREATE TABLE IF NOT EXISTS replica_probe (
		id INT PRIMARY KEY,
		note TEXT NOT NULL
	)`
	s.Require().NoError(dbConn.Exec(createSQL).Error)

	// Idempotent across re-runs against a reused container.
	s.Require().NoError(
		dbConn.Clauses(dbresolver.Write).
			Exec(`INSERT INTO replica_probe (id, note) VALUES (1, 'hello-replica')
			      ON CONFLICT (id) DO UPDATE SET note = EXCLUDED.note`).
			Error,
	)

	// WAL streaming is async; poll with a bounded window.
	deadline := time.Now().Add(walCatchupTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		var note string
		lastErr = dbConn.Clauses(dbresolver.Read).
			Raw(`SELECT note FROM replica_probe WHERE id = 1`).
			Scan(&note).Error
		if lastErr == nil && note == "hello-replica" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.Failf("replica did not catch up",
		"row not visible on standby within %s (last err: %v)", walCatchupTimeout, lastErr)
}

// TestTransactionPinsToPrimary locks down the dbresolver semantic that
// underpins CMK's current behaviour: opening a transaction without
// dbresolver.Read pins reads inside it to the primary, even if a later
// Clauses(Read) is applied. multitenancy.WithTenant follows this path.
func (s *PSQLReplicaSuite) TestTransactionPinsToPrimary() {
	dbConn := s.openConn()

	// Plain transaction — what multitenancy.WithTenant does under the hood.
	s.Require().NoError(dbConn.Transaction(func(tx *multitenancy.DB) error {
		s.False(onReplica(s.T(), tx.DB),
			"plain Transaction reads must land on primary (got recovery=true)")
		// Inside-tx clauses cannot escape the pinned connection.
		s.False(onReplica(s.T(), tx.Clauses(dbresolver.Read)),
			"inside-tx Clauses(Read) must still hit primary; the connection is already pinned")
		return nil
	}))

	// Explicit Read-clause transaction routes to standby.
	s.Require().NoError(dbConn.Clauses(dbresolver.Read).Transaction(func(tx *gorm.DB) error {
		s.True(onReplica(s.T(), tx),
			"Clauses(Read).Transaction reads must land on standby")
		return nil
	}))
}

// TestMultitenancyWithTenantPinsToPrimary exercises the exact library call
// CMK's repo layer makes: reads inside the closure land on the primary
// even though replicas are configured.
func (s *PSQLReplicaSuite) TestMultitenancyWithTenantPinsToPrimary() {
	dbConn := s.openConn()

	// Throwaway schema — avoids pulling in full CMK migrations.
	const tenant = "psqlreplicatests_probe"
	s.Require().NoError(dbConn.Exec(`CREATE SCHEMA IF NOT EXISTS ` + tenant).Error)
	s.T().Cleanup(func() {
		// tb.Context() is cancelled before cleanups; use a fresh one.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = dbConn.WithContext(ctx).Exec(`DROP SCHEMA IF EXISTS ` + tenant + ` CASCADE`).Error
	})

	err := dbConn.WithTenant(s.T().Context(), tenant, func(tx *multitenancy.DB) error {
		s.False(onReplica(s.T(), tx.DB),
			"reads inside multitenancy.WithTenant must land on primary")
		return nil
	})
	s.Require().NoError(err)
}

// TestReplicaIsReadOnly guards against a fixture regression where the standby
// comes up writable, which would silently invalidate the other tests.
func (s *PSQLReplicaSuite) TestReplicaIsReadOnly() {
	dbConn := s.openConn()

	err := dbConn.Clauses(dbresolver.Read).
		Exec(`CREATE TABLE replica_should_fail (id INT)`).Error

	s.Require().Error(err, "writing through read clause should hit standby and fail")

	// Assert on SQLSTATE rather than a wire-message substring so a future
	// driver/GORM error-translation change doesn't silently weaken this.
	var pgErr *pgconn.PgError
	s.Require().ErrorAs(err, &pgErr, "expected a *pgconn.PgError, got: %v", err)
	s.Equal(pgReadOnlyTxn, pgErr.Code, "expected SQLSTATE 25006 (read_only_sql_transaction)")
}

// TestEmptyReplicaConfig covers the no-replica branch in db.StartDBConnection.
func (s *PSQLReplicaSuite) TestEmptyReplicaConfig() {
	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.pair.Primary,
		[]config.Database{},
		nil,
	)

	s.Require().NoError(err)
	s.Require().NotNil(dbConn)
}

// TestReplicaConnectionError verifies that a misconfigured replica fails at
// startup rather than silently degrading to a primary-only setup.
func (s *PSQLReplicaSuite) TestReplicaConnectionError() {
	badReplica := s.pair.Replica
	badReplica.Port = "1" // closed on practically every host

	dbConn, err := db.StartDBConnection(
		s.T().Context(),
		s.pair.Primary,
		[]config.Database{badReplica},
		nil,
	)

	s.Require().Error(err)
	s.Require().Nil(dbConn)
	s.Require().ErrorIs(err, db.ErrDBResolver,
		"expected the resolver-startup sentinel so callers can distinguish replica failures")
}

func TestPSQLReplicaSuite(t *testing.T) {
	suite.Run(t, new(PSQLReplicaSuite))
}
