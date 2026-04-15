package async

import (
	"maps"

	"github.com/hibiken/asynq"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/utils/ptr"
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
		def := tasks[cfg.TaskType]
		if cfg.Enabled != nil {
			def.Enabled = cfg.Enabled
		}
		if cfg.Cronspec != "" {
			def.Cronspec = cfg.Cronspec
		}
		if cfg.Retries != nil {
			def.Retries = cfg.Retries
		}
		tasks[cfg.TaskType] = def
	}

	configs := make([]*asynq.PeriodicTaskConfig, 0)
	for name, cfg := range tasks {
		if cfg.Enabled != nil && !*cfg.Enabled {
			continue
		}
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: cfg.Cronspec,
			Task:     asynq.NewTask(name, nil, asynq.MaxRetry(ptr.GetIntOrDefault(cfg.Retries, 0))),
		})
	}

	return configs, nil
}
