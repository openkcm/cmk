package db

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dialect"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrStartingDBCon            = errors.New("error starting db connection")
	ErrDBResolver               = errors.New("error starting db resolver")
	ErrLoadingDsnFromDBConfig   = errors.New("error loading dsn from db config")
	ErrLoadingReplicaDialectors = errors.New("error loading replica dialectors")
)

// StartDBConnection opens db connection using data from `config.db`.
func StartDBConnection(
	conf config.Database,
	replicas []config.Database,
) (*multitenancy.DB, error) {
	return StartDBConnectionPlugins(conf, replicas, map[string]gorm.Plugin{})
}

// StartDBConnectionPlugins opens db connection using data from `config.db`
// and plugins that are passed in a form of map because GORM config stores
// them this way.
// It is an extension of `StartDBConnection` functionality.
func StartDBConnectionPlugins(
	conf config.Database,
	replicas []config.Database,
	plugins map[string]gorm.Plugin,
) (*multitenancy.DB, error) {
	dsnFromConfig, err := dsn.FromDBConfig(conf)
	if err != nil {
		return nil, errs.Wrap(ErrLoadingDsnFromDBConfig, err)
	}

	dialector := dialect.NewFrom(dsnFromConfig)

	db, err := multitenancy.Open(dialector, &gorm.Config{
		Plugins:        plugins,
		TranslateError: true,
	})
	if err != nil {
		return nil, errs.Wrap(ErrStartingDBCon, err)
	}

	if len(replicas) == 0 {
		return db, nil
	}

	replicaDialectorsFromReplicas, err := replicaDialectors(replicas)
	if err != nil {
		return nil, errs.Wrap(ErrLoadingReplicaDialectors, err)
	}

	err = db.Use(dbresolver.Register(dbresolver.Config{
		Sources:  []gorm.Dialector{dialector},
		Replicas: replicaDialectorsFromReplicas,
		Policy:   dbresolver.RandomPolicy{},
	}))
	if err != nil {
		return nil, errs.Wrap(ErrDBResolver, err)
	}

	return db, nil
}

func replicaDialectors(replicas []config.Database) ([]gorm.Dialector, error) {
	dialects := make([]gorm.Dialector, 0, len(replicas))

	for _, r := range replicas {
		dsnFromConfig, err := dsn.FromDBConfig(r)
		if err != nil {
			return nil, errs.Wrap(ErrLoadingDsnFromDBConfig, err)
		}

		dialects = append(dialects, dialect.NewFrom(dsnFromConfig))
	}

	return dialects, nil
}
