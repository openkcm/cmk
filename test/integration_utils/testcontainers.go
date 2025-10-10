package integrationutils

import (
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

const (
	rabbitMQContainer = "testcontainers-rabbitmq"
	postgresContainer = "testcontainers-postgresql"
)

func StartRabbitMQ(tb testing.TB) *rabbitmq.RabbitMQContainer {
	tb.Helper()

	service, err := rabbitmq.Run(
		tb.Context(),
		"rabbitmq:4-alpine",
		testcontainers.WithReuseByName(rabbitMQContainer),
	)
	assert.NoError(tb, err)

	return service
}

func StartPostgresSQL(tb testing.TB) (*postgres.PostgresContainer, string) {
	tb.Helper()

	service, err := postgres.Run(tb.Context(),
		"postgres:16-alpine",
		testcontainers.WithReuseByName(postgresContainer),
		postgres.WithDatabase(DB.Name),
		postgres.WithUsername(DB.User.Value),
		postgres.WithPassword(DB.Secret.Value),
		postgres.BasicWaitStrategies(),
	)
	assert.NoError(tb, err)

	p, err := service.MappedPort(tb.Context(), nat.Port("5432"))
	assert.NoError(tb, err)

	return service, p.Port()
}
