package async

import (
	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk-core/internal/config"
)

// ScheduledTaskConfigProvider implements asynq PeriodicTaskConfigProvider interface.
type ScheduledTaskConfigProvider struct {
	Config *config.Config
}

// GetConfigs Parses the yaml file and return a list of PeriodicTaskConfigs.
func (p *ScheduledTaskConfigProvider) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	tasks := p.Config.Scheduler.Tasks

	configs := make([]*asynq.PeriodicTaskConfig, len(tasks))
	for i, cfg := range tasks {
		configs[i] = &asynq.PeriodicTaskConfig{
			Cronspec: cfg.Cronspec,
			Task: asynq.NewTask(
				cfg.TaskType,
				nil,
				asynq.MaxRetry(cfg.Retries),
			),
		}
	}

	return configs, nil
}
