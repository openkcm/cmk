package testutils

import (
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/openkcm/cmk/internal/config"
)

const (
	rabbitMQContainer = "testcontainers-rabbitmq-shared"
	postgresContainer = "testcontainers-postgresql-shared"
	redisContainer    = "testcontainers-redis-shared"
)

func StartRabbitMQ(
	tb testing.TB,
	opts ...testcontainers.ContainerCustomizer,
) string {
	tb.Helper()

	options := append([]testcontainers.ContainerCustomizer{
		testcontainers.WithReuseByName(rabbitMQContainer),
		testcontainers.WithAdditionalWaitStrategy(wait.ForListeningPort(nat.Port(rabbitmq.DefaultAMQPPort))),
	}, opts...)

	service, err := rabbitmq.Run(
		tb.Context(),
		"rabbitmq:4-alpine",
		options...,
	)
	assert.NoError(tb, err)

	url, err := service.AmqpURL(tb.Context())
	assert.NoError(tb, err)

	return url
}

func StartPostgresSQL(
	tb testing.TB,
	cfg *config.Database,
	opts ...testcontainers.ContainerCustomizer,
) {
	tb.Helper()

	var name string
	var user, secret commoncfg.SourceRef
	if cfg != nil && *cfg != (config.Database{}) {
		name = cfg.Name
		user = cfg.User
		secret = cfg.Secret
	} else {
		name = TestDB.Name
		user = TestDB.User
		secret = TestDB.Secret
	}

	// Do it like this so the user specified override the defaults
	options := append([]testcontainers.ContainerCustomizer{
		postgres.WithDatabase(name),
		postgres.WithUsername(user.Value),
		postgres.WithPassword(secret.Value),
		postgres.BasicWaitStrategies(),
		testcontainers.WithStartupCommand(testcontainers.NewRawCommand([]string{
			"postgres",
			"-c", "max_connections=1000",
		})),
		testcontainers.WithReuseByName(postgresContainer),
	}, opts...)

	service, err := postgres.Run(tb.Context(),
		"postgres:16-alpine",
		options...,
	)
	assert.NoError(tb, err)

	if cfg != nil {
		p, err := service.MappedPort(tb.Context(), nat.Port("5432"))
		assert.NoError(tb, err)

		host, err := service.Host(tb.Context())
		assert.NoError(tb, err)

		cfg.Port = p.Port()
		cfg.Name = name
		cfg.User = user
		cfg.Secret = secret
		cfg.Host = commoncfg.SourceRef{
			Value:  host,
			Source: commoncfg.EmbeddedSourceValue,
		}
	}
}

func StartRedis(
	tb testing.TB,
	cfg *config.Scheduler,
	opts ...testcontainers.ContainerCustomizer,
) {
	tb.Helper()

	// Do it like this so the user specified override the defaults
	options := append([]testcontainers.ContainerCustomizer{
		testcontainers.WithReuseByName(redisContainer),
	}, opts...)

	redisContainer, err := redis.Run(tb.Context(),
		"redis:7",
		options...,
	)

	assert.NoError(tb, err)

	if cfg != nil {
		port, err := redisContainer.MappedPort(tb.Context(), nat.Port("6379"))
		assert.NoError(tb, err)

		cfg.TaskQueue.Port = port.Port()
	}
}
