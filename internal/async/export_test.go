package async

import "github.com/hibiken/asynq"

func (a *App) GetTaskQueueCfg() asynq.RedisClientOpt {
	return a.taskQueueCfg
}

func (a *App) GetFanOutOpts(taskType string) (bool, []asynq.Option) {
	return a.getFanOutOpts(taskType)
}
