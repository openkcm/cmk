package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/pressly/goose/v3"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	shareddatamigrations "github.com/openkcm/cmk/migrations/shared/data"
	tenantdatamigrations "github.com/openkcm/cmk/migrations/tenant/data"
)

var ErrCommandUnsuported = errors.New("command not supported")

type (
	MigrationType      string
	MigrationTarget    string
	migrateFunc        func(provider *goose.Provider) ([]*goose.MigrationResult, error)
	MigrationResultMap map[string][]*goose.MigrationResult // Map[schema]migrationResults
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
	MigrateTenantToLatest(ctx context.Context, tenant *model.Tenant) ([]*goose.MigrationResult, error)
	MigrateToLatest(ctx context.Context, migration Migration) (MigrationResultMap, error)
	MigrateTo(ctx context.Context, migration Migration, version int64) (MigrationResultMap, error)
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
) (MigrationResultMap, error) {
	return m.migrate(ctx, migration, func(gp *goose.Provider) ([]*goose.MigrationResult, error) {
		if migration.Downgrade {
			r, err := gp.Down(ctx)
			return []*goose.MigrationResult{r}, err
		}
		return gp.Up(ctx)
	})
}

// MigrateTo runs migrations up-to a specific version
// For migrations with Downgrade false, it migrates up to the specified version
// For migrations with Downgrade true, it downgrades until the DB is the specified version
func (m *migrator) MigrateTo(
	ctx context.Context,
	migration Migration,
	version int64,
) (MigrationResultMap, error) {
	return m.migrate(ctx, migration, func(gp *goose.Provider) ([]*goose.MigrationResult, error) {
		if migration.Downgrade {
			return gp.DownTo(ctx, version)
		}
		return gp.UpTo(ctx, version)
	})
}

// MigrateTenantToLatest runs schema migrations on the tenant up to the latest version
// It's inteded to be used only whenever creating new tenants
func (m *migrator) MigrateTenantToLatest(
	ctx context.Context,
	tenant *model.Tenant,
) ([]*goose.MigrationResult, error) {
	mig := Migration{
		Downgrade: false,
		Type:      SchemaMigration,
		Target:    TenantTarget,
	}

	return m.runMigration(
		ctx,
		mig,
		tenant.SchemaName,
		func(p *goose.Provider) ([]*goose.MigrationResult, error) {
			return p.Up(ctx)
		},
	)
}

func (m *migrator) migrate(
	ctx context.Context,
	migration Migration,
	f migrateFunc,
) (MigrationResultMap, error) {
	switch migration.Target {
	case SharedTarget:
		res, err := m.runMigration(ctx, migration, SharedSchema, f)
		if err != nil {
			return nil, err
		}
		return MigrationResultMap{
			SharedSchema: res,
		}, nil
	case TenantTarget:
		return m.migrateTenants(ctx, migration, func(p *goose.Provider) ([]*goose.MigrationResult, error) {
			return f(p)
		})
	case AllTarget:
		migration.Target = SharedTarget
		sharedRes, err := m.runMigration(ctx, migration, SharedSchema, f)
		if err != nil {
			return nil, err
		}
		migration.Target = TenantTarget
		tenantsRes, err := m.migrateTenants(ctx, migration, func(p *goose.Provider) ([]*goose.MigrationResult, error) {
			return f(p)
		})
		if err != nil {
			return MigrationResultMap{
				SharedSchema: sharedRes,
			}, err
		}
		tenantsRes[SharedSchema] = sharedRes
		return tenantsRes, nil
	default:
		return nil, ErrUnsupportedMigration
	}
}

func (m *migrator) migrateTenants(
	ctx context.Context,
	migration Migration,
	f migrateFunc,
) (MigrationResultMap, error) {
	res := make(MigrationResultMap)
	err := repo.ProcessInBatch(ctx, m.r, repo.NewQuery(), repo.DefaultLimit, func(tenants []*model.Tenant) error {
		for _, t := range tenants {
			iRes, err := m.runMigration(ctx, migration, t.SchemaName, f)
			if err != nil {
				return err
			}
			res[t.ID] = iRes
		}
		return nil
	})
	return res, err
}

func (m *migrator) runMigration(
	ctx context.Context,
	mig Migration,
	schema string,
	f func(*goose.Provider) ([]*goose.MigrationResult, error),
) ([]*goose.MigrationResult, error) {
	dbCon, err := m.newSchemaDBCon(schema)
	if err != nil {
		return nil, err
	}
	defer dbCon.Close()

	ctx = slogctx.With(ctx,
		slog.String("Schema", schema),
		slog.Any("Migration", mig),
	)

	if mig.Target == TenantTarget {
		err := ValidateSchema(schema)
		if err != nil {
			return nil, err
		}

		query := "CREATE SCHEMA IF NOT EXISTS " + quoteSchema(schema)

		// NOSONAR is required here because DDL statements cannot be parameterized.
		// The schema variable is strictly validated and quoted prior to this execution.
		_, err = dbCon.ExecContext(ctx, query) // NOSONAR
		if err != nil {
			return nil, err
		}
	}

	provider, err := m.newGooseProvider(dbCon, mig)
	if errors.Is(err, goose.ErrNoMigrations) {
		log.Warn(ctx, "No migration files found, skipping")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return f(provider)
}

func (m *migrator) newGooseProvider(dbCon *sql.DB, mig Migration) (*goose.Provider, error) {
	var versionTable string
	var goMigrations []*goose.Migration
	switch mig.Type {
	case DataMigration:
		versionTable = DataMigrationTable
		if mig.Target == TenantTarget {
			goMigrations = tenantdatamigrations.GetMigrations()
		} else {
			goMigrations = shareddatamigrations.GetMigrations()
		}
	case SchemaMigration:
		versionTable = SchemaMigrationTable
	default:
		return nil, ErrUnsupportedMigration
	}

	dir, err := m.getMigrationDir(mig)
	if err != nil {
		return nil, err
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		dbCon,
		os.DirFS(dir),
		goose.WithTableName(versionTable),
		goose.WithGoMigrations(goMigrations...),
	)

	return provider, err
}

func (m *migrator) newSchemaDBCon(
	schema string,
) (*sql.DB, error) {
	schema = quoteSchema(schema)

	dsn := fmt.Sprintf("%s search_path=%s", m.dsn, schema)
	db, err := goose.OpenDBWithDriver(string(goose.DialectPostgres), dsn)

	return db, err
}

func quoteSchema(schema string) string {
	return fmt.Sprintf(`"%s"`, schema)
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
