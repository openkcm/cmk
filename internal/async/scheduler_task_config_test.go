package async_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/async"
	"github.com/openkcm/cmk-core/internal/config"
)

func TestGetConfigs(t *testing.T) {
	t.Run("Should get config file", func(t *testing.T) {
		f := async.ScheduledTaskConfigProvider{Config: &config.Config{
			Scheduler: config.Scheduler{
				Tasks: []config.Task{{
					TaskType: config.TypeSystemsTask,
				}},
			},
		}}

		schedulerTasks, _ := f.GetConfigs()
		for _, task := range schedulerTasks {
			assert.Contains(t, task.Task.Type(), config.TypeSystemsTask)
		}
	})
}
