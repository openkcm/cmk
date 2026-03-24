package dsn

import (
	"errors"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrLoadingDatabaseHost     = errors.New("error loading database host")
	ErrLoadingDatabaseUser     = errors.New("error loading database user")
	ErrLoadingDatabasePassword = errors.New("error loading database password")
)

// FromDBConfig converts `config.Database` data to a DSN and returns it.
func FromDBConfig(conf config.Database) (string, error) {
	host, err := commoncfg.LoadValueFromSourceRef(conf.Host)
	if err != nil {
		return "", errs.Wrap(ErrLoadingDatabaseHost, err)
	}

	user, err := commoncfg.LoadValueFromSourceRef(conf.User)
	if err != nil {
		return "", errs.Wrap(ErrLoadingDatabaseUser, err)
	}

	password, err := commoncfg.LoadValueFromSourceRef(conf.Secret)
	if err != nil {
		return "", errs.Wrap(ErrLoadingDatabasePassword, err)
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, string(password), conf.Name, conf.Port, conf.Parameters.SSL.Mode)

	if conf.Parameters.SSL.RootCert != "" {
		dsn = fmt.Sprintf("%s sslrootcert=%s", dsn, conf.Parameters.SSL.RootCert)
	}
	if conf.Parameters.SSL.Cert != "" {
		dsn = fmt.Sprintf("%s sslcert=%s", dsn, conf.Parameters.SSL.Cert)
	}
	if conf.Parameters.SSL.Key != "" {
		dsn = fmt.Sprintf("%s sslkey=%s", dsn, conf.Parameters.SSL.Key)
	}

	return dsn, nil
}
