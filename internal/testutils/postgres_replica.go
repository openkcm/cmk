package testutils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	tcnetwork "github.com/testcontainers/testcontainers-go/network"

	"github.com/openkcm/cmk/internal/config"
)

const (
	primaryAlias        = "primary"
	standbyAlias        = "standby"
	replicaSetupTimeout = 90 * time.Second
	cleanupTimeout      = 15 * time.Second
)

// primaryPostgresConf enables physical streaming replication on the primary.
const primaryPostgresConf = `listen_addresses = '*'
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10
hot_standby = on
`

// primaryPgHba grants replication access to the replicator role.
//
//nolint:dupword // "all all" is the pg_hba.conf row syntax (user user).
const primaryPgHba = `local   all             all                                     trust
host    all             all             0.0.0.0/0               md5
host    all             all             ::/0                    md5
host    replication     replicator      0.0.0.0/0               md5
host    replication     replicator      ::/0                    md5
`

// primaryInitSQLTemplate creates the replication role; runs via
// /docker-entrypoint-initdb.d.
const primaryInitSQLTemplate = `CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD '%s';
`

// standbyEntrypointTemplate clones the primary via pg_basebackup and execs
// postgres directly. The official docker-entrypoint.sh would crash the
// read-only standby by attempting a password-reset SQL on startup.
//
//nolint:dupword // "exec su-exec postgres postgres" is correct: command + user + binary.
const standbyEntrypointTemplate = `#!/bin/bash
set -euo pipefail

PGDATA="${PGDATA:-/var/lib/postgresql/data}"
mkdir -p "$PGDATA"
chown -R postgres:postgres "$PGDATA"
chmod 0700 "$PGDATA"

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  export PGPASSWORD='%s'
  until su-exec postgres pg_basebackup -h primary -p 5432 -U replicator -D "$PGDATA" -Fp -Xs -P -R; do
    echo "pg_basebackup not ready yet, retrying..."
    sleep 1
  done
fi

exec su-exec postgres postgres -D "$PGDATA"
`

// ReplicaPair holds the primary and standby configs returned by StartPostgresWithReplica.
type ReplicaPair struct {
	Primary config.Database
	Replica config.Database
}

// StartPostgresWithReplica boots a PostgreSQL primary + streaming standby.
// Always creates fresh containers; share one pair per suite via SetupSuite.
// The returned configs are ready to pass to db.StartDBConnection.
func StartPostgresWithReplica(tb testing.TB) ReplicaPair {
	tb.Helper()

	ctx := tb.Context()

	const (
		dbName = "cmk"
		dbUser = "postgres"
	)

	// Generated at runtime to keep credentials out of source.
	dbPassword := randHex(tb, 16)
	replicatorPassword := randHex(tb, 16)

	nw, err := tcnetwork.New(ctx)
	require.NoError(tb, err, "create docker network for replica pair")
	tb.Cleanup(func() {
		// tb.Context() is cancelled before cleanups; use a fresh one.
		c, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := nw.Remove(c); err != nil {
			tb.Logf("warning: remove docker network %s: %v", nw.ID, err)
		}
	})

	confDir := tb.TempDir()
	primary := startReplicaPrimary(tb, ctx, nw, confDir, dbName, dbUser, dbPassword, replicatorPassword)
	standby := startReplicaStandby(tb, ctx, nw, confDir, dbName, dbUser, dbPassword, replicatorPassword)

	return ReplicaPair{
		Primary: buildDBConfig(tb, primary, dbName, dbUser, dbPassword),
		Replica: buildDBConfig(tb, standby, dbName, dbUser, dbPassword),
	}
}

// startReplicaPrimary boots the primary with replication enabled; mounts
// pg_hba.conf and overrides hba_file via -c.
func startReplicaPrimary(
	tb testing.TB,
	ctx context.Context,
	nw *testcontainers.DockerNetwork,
	confDir, dbName, dbUser, dbPassword, replicatorPassword string,
) *postgres.PostgresContainer {
	tb.Helper()

	writeFile(tb, filepath.Join(confDir, "postgresql.conf"), primaryPostgresConf)
	writeFile(tb, filepath.Join(confDir, "pg_hba.conf"), primaryPgHba)
	writeFile(tb, filepath.Join(confDir, "init-replicator.sql"),
		fmt.Sprintf(primaryInitSQLTemplate, replicatorPassword))

	primary, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.WithConfigFile(filepath.Join(confDir, "postgresql.conf")),
		postgres.WithInitScripts(filepath.Join(confDir, "init-replicator.sql")),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      filepath.Join(confDir, "pg_hba.conf"),
			ContainerFilePath: "/etc/postgresql/pg_hba.conf",
			// 0o644: readable by the non-root postgres user; pg_hba.conf contains no secrets.
			FileMode: 0o644,
		}),
		testcontainers.WithCmdArgs("-c", "hba_file=/etc/postgresql/pg_hba.conf"),
		tcnetwork.WithNetwork([]string{primaryAlias}, nw),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(tb, err, "start primary postgres")
	//nolint:contextcheck // cleanup builds its own ctx by design.
	tb.Cleanup(func() { terminateContainer(tb, primary, "primary") })

	return primary
}

// startReplicaStandby boots the standby. Waits on both the read-only log
// line and TCP 5432 — the log alone was flaky when the standby crashed
// after logging ready but before accepting connections.
func startReplicaStandby(
	tb testing.TB,
	ctx context.Context,
	nw *testcontainers.DockerNetwork,
	confDir, dbName, dbUser, dbPassword, replicatorPassword string,
) *postgres.PostgresContainer {
	tb.Helper()

	scriptPath := filepath.Join(confDir, "replica-entrypoint.sh")
	writeFile(tb, scriptPath,
		fmt.Sprintf(standbyEntrypointTemplate, replicatorPassword))

	standby, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      scriptPath,
			ContainerFilePath: "/usr/local/bin/replica-entrypoint.sh",
			FileMode:          0o755,
		}),
		testcontainers.WithEntrypoint("/usr/local/bin/replica-entrypoint.sh"),
		tcnetwork.WithNetwork([]string{standbyAlias}, nw),
		// Match the outer deadline to replicaSetupTimeout; the default is 60s.
		testcontainers.WithAdditionalWaitStrategyAndDeadline(
			replicaSetupTimeout,
			wait.ForAll(
				wait.ForLog("database system is ready to accept read-only connections").
					WithStartupTimeout(replicaSetupTimeout),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(replicaSetupTimeout),
			),
		),
	)
	require.NoError(tb, err, "start standby postgres")
	//nolint:contextcheck // cleanup builds its own ctx by design.
	tb.Cleanup(func() { terminateContainer(tb, standby, "standby") })

	return standby
}

// buildDBConfig reads the mapped port and host from a container into config.Database.
func buildDBConfig(
	tb testing.TB,
	c *postgres.PostgresContainer,
	name, user, password string,
) config.Database {
	tb.Helper()

	ctx := tb.Context()

	mapped, err := c.MappedPort(ctx, "5432")
	require.NoError(tb, err, "read mapped port")

	host, err := c.Host(ctx)
	require.NoError(tb, err, "read host")

	return config.Database{
		Host: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  host,
		},
		User: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  user,
		},
		Secret: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  password,
		},
		Name: name,
		Port: mapped.Port(),
	}
}

// terminateContainer stops a container with a fresh context — see the
// network cleanup in StartPostgresWithReplica for why.
func terminateContainer(tb testing.TB, c testcontainers.Container, label string) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if err := c.Terminate(ctx); err != nil {
		tb.Logf("warning: terminate %s container: %v", label, err)
	}
}

// randHex returns nBytes of crypto-random data as a hex string. Hex
// avoids quoting hazards when interpolated into SQL or shell.
func randHex(tb testing.TB, nBytes int) string {
	tb.Helper()
	buf := make([]byte, nBytes)
	_, err := rand.Read(buf)
	require.NoError(tb, err, "crypto/rand read")
	return hex.EncodeToString(buf)
}

func writeFile(tb testing.TB, path, content string) {
	tb.Helper()
	// 0o644 so a non-root container user (uid-mapped Linux Docker) can read.
	//nolint:gosec // test fixture; no secrets.
	require.NoError(tb, os.WriteFile(path, []byte(content), 0o644), "write "+path)
}
