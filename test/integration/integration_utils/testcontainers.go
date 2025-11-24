package integrationutils

import (
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/openkcm/cmk/internal/config"
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

func StartPostgresSQL(tb testing.TB, cfg *config.Database) *postgres.PostgresContainer {
	tb.Helper()

	var name, user, secret string
	if cfg != nil {
		name = cfg.Name
		user = cfg.User.Value
		secret = cfg.Secret.Value
	} else {
		name = DB.Name
		user = DB.User.Value
		secret = DB.Secret.Value
	}

	service, err := postgres.Run(tb.Context(),
		"postgres:16-alpine",
		testcontainers.WithReuseByName(postgresContainer),
		postgres.WithDatabase(name),
		postgres.WithUsername(user),
		postgres.WithPassword(secret),
		postgres.BasicWaitStrategies(),
	)
	assert.NoError(tb, err)

	if cfg != nil {
		p, err := service.MappedPort(tb.Context(), nat.Port("5432"))
		assert.NoError(tb, err)

		cfg.Port = p.Port()
	}

	return service
}

func StartRedis(tb testing.TB, cfg *config.Scheduler) *redis.RedisContainer {
	tb.Helper()

	redisContainer, err := redis.Run(tb.Context(),
		"redis:7",
		testcontainers.WithReuseByName("testcontainers-redis"),
	)

	assert.NoError(tb, err)

	if cfg != nil {
		port, err := redisContainer.MappedPort(tb.Context(), nat.Port("6379"))
		assert.NoError(tb, err)

		cfg.TaskQueue.Port = port.Port()
	}

	return redisContainer
}
