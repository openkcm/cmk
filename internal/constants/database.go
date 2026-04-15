package constants

const (
	// DBDriverName is the standard PostgreSQL driver name used for otelsql tracing
	DBDriverName = "postgres"

	// PgxDriverName is the pgx-specific driver name used for health checks and direct connections
	PgxDriverName = "pgx"
)
