package async_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
)

func TestGetConfigs(t *testing.T) {
	t.Run("Default only", func(t *testing.T) {
		original := config.PeriodicTasks
		defer func() { config.PeriodicTasks = original }()

		config.PeriodicTasks = map[string]config.Task{
			"task1": {
				Enabled:  true,
				Cronspec: "*/1 * * * *",
				Retries:  3,
			},
			"task2": {
				Enabled:  false,
				Cronspec: "*/5 * * * *",
				Retries:  0,
			},
		}

		p := async.ScheduledTaskConfigProvider{
			Config: &config.Config{
				Scheduler: config.Scheduler{Tasks: []config.Task{}},
			},
		}

		configs, err := p.GetConfigs()
		assert.NoError(t, err)
		assert.Len(t, configs, 1)
	})

	t.Run("Config overrides default", func(t *testing.T) {
		original := config.PeriodicTasks
		defer func() { config.PeriodicTasks = original }()

		config.PeriodicTasks = map[string]config.Task{
			"task1": {
				Enabled:  true,
				Cronspec: "1 * * * *",
				Retries:  10,
			},
			"task2": {
				Enabled:  true,
				Cronspec: "*/5 * * * *",
				Retries:  0,
			},
		}

		p := async.ScheduledTaskConfigProvider{
			Config: &config.Config{
				Scheduler: config.Scheduler{
					Tasks: []config.Task{
						{
							TaskType: "task1",
							Enabled:  true,
							Cronspec: "30 * * * *",
							Retries:  1,
						},
						{
							TaskType: "task2",
							Enabled:  false,
							Cronspec: "30 * * * *",
							Retries:  1,
						},
					},
				},
			},
		}

		configs, err := p.GetConfigs()
		assert.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, "task1", configs[0].Task.Type())
		assert.Equal(t, "30 * * * *", configs[0].Cronspec)
	})
}
