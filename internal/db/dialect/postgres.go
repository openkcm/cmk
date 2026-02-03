package dialect

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pg "github.com/bartventer/gorm-multitenancy/postgres/v8"
)

// NewFrom returns a postgres dialector.
// Hint: `dsn` package contains utility to convert `config.DB` to DSN string that can be passed here.
// Note: PreferSimpleProtocol is enabled to disable prepared statement caching, which prevents
// "cached plan must not change result type" errors
func NewFrom(dsn string) gorm.Dialector {
	return pg.New(pg.Config{
		Config: postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		},
	})
}
