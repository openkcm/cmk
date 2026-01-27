package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
)

var ErrCommandUnsuported = errors.New("command not supported")

type (
	MigrationType   string
	MigrationTarget string
	migrateFunc     func(ctx context.Context, db *sql.DB, dir string) error
)

const (
	DataMigrationTable                   = "goose_db_data_version"
	SchemaMigrationTable                 = "goose_db_schema_version"
	SharedSchema                         = "public"
	SchemaMigration      MigrationType   = "schema"
	DataMigration        MigrationType   = "data"
	SharedTarget         MigrationTarget = "shared"
	TenantTarget         MigrationTarget = "tenant"
	AllTarget            MigrationTarget = "all"
)

var ErrUnsupportedMigration = errors.New("unsupported migration")

type migrator struct {
	r   repo.Repo
	dsn string
	cfg *config.Config
}

type Migration struct {
	Downgrade bool
	Type      MigrationType
	Target    MigrationTarget
}

type Migrator interface {
	MigrateTenantToLatest(ctx context.Context, tenant *model.Tenant) error
	MigrateToLatest(ctx context.Context, migration Migration) error
	MigrateTo(ctx context.Context, migration Migration, version int64) error
}

func NewMigrator(r repo.Repo, cfg *config.Config) (Migrator, error) {
	dsn, err := dsn.FromDBConfig(cfg.Database)
	if err != nil {
		return nil, err
	}

	return &migrator{
		r:   r,
		dsn: dsn,
		cfg: cfg,
	}, nil
}

// MigrateToLatest runs migrations onto the latest version
// For migrations with Downgrade false, it runs all migrations up to and including the latest version
// For migrations with Downgrade true, it downgrades the latest version
func (m *migrator) MigrateToLatest(
	ctx context.Context,
	migration Migration,
) error {
	return m.migrate(ctx, migration, func(ctx context.Context, db *sql.DB, dir string) error {
		if migration.Downgrade {
			return goose.DownContext(ctx, db, dir)
		}
		return goose.UpContext(ctx, db, dir)
	})
}

// MigrateTo runs migrations up-to a specific version
// For migrations with Downgrade false, it migrates up to the specified version
// For migrations with Downgrade true, it downgrades until the DB is the specified version
func (m *migrator) MigrateTo(
	ctx context.Context,
	migration Migration,
	version int64,
) error {
	return m.migrate(ctx, migration, func(ctx context.Context, db *sql.DB, dir string) error {
		if migration.Downgrade {
			return goose.DownToContext(ctx, db, dir, version)
		}
		return goose.UpToContext(ctx, db, dir, version)
	})
}

// MigrateTenantToLatest runs schema migrations on the tenant up to the latest version
// It's inteded to be used only whenever creating new tenants
func (m *migrator) MigrateTenantToLatest(ctx context.Context, tenant *model.Tenant) error {
	mig := Migration{
		Downgrade: false,
		Type:      SchemaMigration,
		Target:    TenantTarget,
	}

	return m.migrateTenant(ctx, mig, tenant, func(ctx context.Context, db *sql.DB, dir string) error {
		return goose.UpContext(ctx, db, dir)
	})
}

func (m *migrator) migrate(
	ctx context.Context,
	migration Migration,
	migrateFunc migrateFunc,
) error {
	switch migration.Target {
	case SharedTarget:
		return m.runMigration(ctx, migration, SharedSchema, func(ctx context.Context, db *sql.DB, dir string) error {
			return migrateFunc(ctx, db, dir)
		})
	case TenantTarget:
		return m.migrateTenants(ctx, migration, func(ctx context.Context, db *sql.DB, dir string) error {
			return migrateFunc(ctx, db, dir)
		})
	case AllTarget:
		mig := migration
		mig.Target = SharedTarget
		err := m.runMigration(ctx, mig, SharedSchema, func(ctx context.Context, db *sql.DB, dir string) error {
			return migrateFunc(ctx, db, dir)
		})
		if err != nil {
			return err
		}
		mig.Target = TenantTarget
		return m.migrateTenants(ctx, mig, func(ctx context.Context, db *sql.DB, dir string) error {
			return migrateFunc(ctx, db, dir)
		})
	default:
		return ErrUnsupportedMigration
	}
}

func (m *migrator) migrateTenants(
	ctx context.Context,
	migration Migration,
	f migrateFunc,
) error {
	return repo.ProcessInBatch(ctx, m.r, repo.NewQuery(), repo.DefaultLimit, func(tenants []*model.Tenant) error {
		for _, t := range tenants {
			err := m.migrateTenant(ctx, migration, t, f)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *migrator) migrateTenant(
	ctx context.Context,
	migration Migration,
	t *model.Tenant,
	f migrateFunc,
) error {
	return m.runMigration(ctx, migration, t.SchemaName, func(ctx context.Context, db *sql.DB, dir string) error {
		_, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+QuoteSchema(t.SchemaName))
		if err != nil {
			return err
		}
		return f(ctx, db, dir)
	})
}

func (m *migrator) runMigration(
	ctx context.Context,
	migration Migration,
	schema string,
	f migrateFunc,
) error {
	dbCon, err := m.newSchemaDBCon(migration, schema)
	if err != nil {
		return err
	}
	defer dbCon.Close()
	dir, err := m.getMigrationDir(migration)
	if err != nil {
		return err
	}
	return f(ctx, dbCon, dir)
}

func (m *migrator) newSchemaDBCon(
	migration Migration,
	schema string,
) (*sql.DB, error) {
	schema = QuoteSchema(schema)

	dsn := fmt.Sprintf("%s search_path=%s", m.dsn, schema)
	db, err := goose.OpenDBWithDriver(string(goose.DialectPostgres), dsn)
	if err != nil {
		return nil, err
	}

	switch migration.Type {
	case DataMigration:
		schema = fmt.Sprintf("%s.%s", schema, DataMigrationTable)
	case SchemaMigration:
		schema = fmt.Sprintf("%s.%s", schema, SchemaMigrationTable)
	default:
		return nil, ErrUnsupportedMigration
	}

	goose.SetTableName(schema)

	return db, nil
}

func QuoteSchema(schema string) string {
	return fmt.Sprintf("\"%s\"", schema)
}

func (m *migrator) getMigrationDir(mig Migration) (string, error) {
	switch {
	case mig.Type == SchemaMigration && mig.Target == SharedTarget:
		return m.cfg.Database.Migrator.Shared.Schema, nil
	case mig.Type == SchemaMigration && mig.Target == TenantTarget:
		return m.cfg.Database.Migrator.Tenant.Schema, nil
	case mig.Type == DataMigration && mig.Target == SharedTarget:
		return m.cfg.Database.Migrator.Shared.Data, nil
	case mig.Type == DataMigration && mig.Target == TenantTarget:
		return m.cfg.Database.Migrator.Tenant.Data, nil
	default:
		return "", ErrUnsupportedMigration
	}
}
