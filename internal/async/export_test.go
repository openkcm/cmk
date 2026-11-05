package async

import (
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func (a *App) GetRedisClient() redis.UniversalClient {
	return a.redisClient
}

func (a *App) GetFanOutOpts(taskType string) (bool, []asynq.Option) {
	return a.getFanOutOpts(taskType)
}

var BuildRedisConnOpt = buildRedisConnOpt
