package config

const (
	TypeSystemsTask        = "sys:refresh"
	TypeCertificateTask    = "cert:rotate"
	TypeHYOKSync           = "key:sync"
	TypeKeystorePool       = "keystore:fill"
	TypeSendNotifications  = "notify:send"
	TypeWorkflowAutoAssign = "workflow:auto-assign"
	TypeWorkflowCleanup    = "workflow:cleanup"
	TypeWorkflowExpire     = "workflow:expire"
)

const defaultRetryCount = 3

// PeriodicTasks defines the periodic tasks with their default configurations.
var PeriodicTasks = map[string]Task{
	TypeSystemsTask: {
		Enabled:  true,
		Cronspec: "0 * * * *", // Hourly
		Retries:  defaultRetryCount,
	},
	TypeHYOKSync: {
		Enabled:  true,
		Cronspec: "0 * * * *", // Hourly
		Retries:  defaultRetryCount,
	},
	TypeKeystorePool: {
		Enabled:  true,
		Cronspec: "0 * * * *", // Hourly
		Retries:  defaultRetryCount,
	},
	TypeCertificateTask: {
		Enabled:  true,
		Cronspec: "0 0 * * *", // At 00:00 AM daily
		Retries:  defaultRetryCount,
	},
	TypeWorkflowExpire: {
		Enabled:  true,
		Cronspec: "0 1 * * *", // At 01:00 AM daily
		Retries:  defaultRetryCount,
	},
	TypeWorkflowCleanup: {
		Enabled:  true,
		Cronspec: "0 2 * * *", // At 02:00 AM daily
		Retries:  defaultRetryCount,
	},
}
