package violations

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgUniqueViolationErrCode = "23505" // see https://www.postgresql.org/docs/14/errcodes-appendix.html
)

// IsUniqueConstraint checks if the error is a PostgreSQL unique constraint violation
func IsUniqueConstraint(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == pgUniqueViolationErrCode
}
