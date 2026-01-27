package async

import (
	"maps"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
)

// ScheduledTaskConfigProvider implements asynq PeriodicTaskConfigProvider interface.
type ScheduledTaskConfigProvider struct {
	Config *config.Config
}

// GetConfigs Parses the yaml file and return a list of PeriodicTaskConfigs.
func (p *ScheduledTaskConfigProvider) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	tasks := make(map[string]config.Task)
	maps.Copy(tasks, config.PeriodicTasks)

	for _, cfg := range p.Config.Scheduler.Tasks {
		tasks[cfg.TaskType] = cfg
	}

	configs := make([]*asynq.PeriodicTaskConfig, 0)
	for name, cfg := range tasks {
		if !cfg.Enabled {
			continue
		}
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: cfg.Cronspec,
			Task:     asynq.NewTask(name, nil, asynq.MaxRetry(cfg.Retries)),
		})
	}

	return configs, nil
}
