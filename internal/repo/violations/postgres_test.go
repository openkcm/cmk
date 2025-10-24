package violations_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/repo/violations"
)

var errNotPostgres = errors.New("not postgres")

// TestIsUniqueConstraint_postgres tests the IsUniqueConstraint function
func TestIsUniqueConstraint_postgres(t *testing.T) {
	t.Run("should return false when error is not a postgres error ", func(t *testing.T) {
		violated := violations.IsUniqueConstraint(errNotPostgres)

		require.False(t, violated)
	})

	t.Run("should return true when error is a postgres error ", func(t *testing.T) {
		postgresErr := &pgconn.PgError{
			Code: violations.PgUniqueErrCode,
		}

		violated := violations.IsUniqueConstraint(postgresErr)

		require.True(t, violated)
	})
}
