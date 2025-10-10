package async

import "github.com/hibiken/asynq"

func (a *App) GetTaskQueueCfg() asynq.RedisClientOpt {
	return a.taskQueueCfg
}
