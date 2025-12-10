package dsn

import (
	"errors"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/errs"
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

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s",
		host, user, string(password), conf.Name, conf.Port), nil
}
