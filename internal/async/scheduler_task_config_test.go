package async_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestGetConfigs(t *testing.T) {
	t.Run("Default only", func(t *testing.T) {
		original := config.PeriodicTasks
		defer func() { config.PeriodicTasks = original }()

		config.PeriodicTasks = map[string]config.Task{
			"task1": {
				Enabled:  ptr.PointTo(true),
				Cronspec: "*/1 * * * *",
				Retries:  ptr.PointTo(3),
			},
			"task2": {
				Enabled:  ptr.PointTo(false),
				Cronspec: "*/5 * * * *",
				Retries:  ptr.PointTo(0),
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
				Enabled:  ptr.PointTo(true),
				Cronspec: "1 * * * *",
				Retries:  ptr.PointTo(10),
			},
			"task2": {
				Enabled:  ptr.PointTo(true),
				Cronspec: "*/5 * * * *",
				Retries:  ptr.PointTo(0),
			},
		}

		p := async.ScheduledTaskConfigProvider{
			Config: &config.Config{
				Scheduler: config.Scheduler{
					Tasks: []config.Task{
						{
							TaskType: "task1",
							Enabled:  ptr.PointTo(true),
							Cronspec: "30 * * * *",
							Retries:  ptr.PointTo(1),
						},
						{
							TaskType: "task2",
							Enabled:  ptr.PointTo(false),
							Cronspec: "30 * * * *",
							Retries:  ptr.PointTo(1),
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

	t.Run("Config without enabled field defaults enabled to true", func(t *testing.T) {
		original := config.PeriodicTasks
		defer func() { config.PeriodicTasks = original }()

		config.PeriodicTasks = map[string]config.Task{
			"task1": {
				Enabled:  ptr.PointTo(true),
				Cronspec: "1 * * * *",
				Retries:  ptr.PointTo(3),
			},
		}

		p := async.ScheduledTaskConfigProvider{
			Config: &config.Config{
				Scheduler: config.Scheduler{
					Tasks: []config.Task{
						{
							// Enabled is nil — omitted from YAML, should keep default true
							TaskType: "task1",
							Cronspec: "30 * * * *",
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
