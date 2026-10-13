package postgres

import (
	"strings"

	"gorm.io/gorm/clause"
)

// toQuoter is implemented by anything that can quote a SQL identifier, notably [gorm.DB].
type toQuoter interface {
	QuoteTo(writer clause.Writer, str string)
}

// quoteRawSQLForTenant builds a raw SQL string by appending a safely-quoted tenant
// identifier to the given prefix, using the dialect's own identifier quoter to prevent
// SQL injection via the tenant/schema name.
func quoteRawSQLForTenant(q toQuoter, raw, tenantID string) string {
	sql := new(strings.Builder)
	_, _ = sql.WriteString(raw)
	q.QuoteTo(sql, tenantID)
	return sql.String()
}
