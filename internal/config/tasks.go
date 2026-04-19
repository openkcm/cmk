package config

import (
	"time"

	"github.com/openkcm/cmk/utils/ptr"
)

const (
	TypeSystemsTask        = "sys:refresh"
	TypeCertificateTask    = "cert:rotate"
	TypeHYOKSync           = "key:sync"
	TypeKeystorePool       = "keystore:fill"
	TypeSendNotifications  = "notify:send"
	TypeWorkflowAutoAssign = "workflow:auto-assign"
	TypeWorkflowCleanup    = "workflow:cleanup"
	TypeWorkflowExpire     = "workflow:expire"
	TypeTenantRefreshName  = "tenant:refresh-name"
)

const defaultRetryCount = 3

// PeriodicTasks defines the periodic tasks with their default configurations.
var PeriodicTasks = map[string]Task{
	TypeSystemsTask: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 * * * *", // Hourly
		Retries:  ptr.PointTo(defaultRetryCount),
	},
	TypeHYOKSync: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "*/5 * * * *", // Every 5 minutes
		Retries:  ptr.PointTo(defaultRetryCount),
		TimeOut:  5 * time.Minute,
		FanOutTask: &FanOutTask{
			Enabled: true,
			Retries: ptr.PointTo(0),
			TimeOut: 5 * time.Minute,
		},
	},
	TypeKeystorePool: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 * * * *", // Hourly
		Retries:  ptr.PointTo(defaultRetryCount),
	},
	TypeCertificateTask: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 0 * * *", // At 00:00 AM daily
		Retries:  ptr.PointTo(defaultRetryCount),
	},
	TypeWorkflowExpire: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 1 * * *", // At 01:00 AM daily
		Retries:  ptr.PointTo(defaultRetryCount),
	},
	TypeWorkflowCleanup: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 2 * * *", // At 02:00 AM daily
		Retries:  ptr.PointTo(defaultRetryCount),
	},
	// The TenantRefreshName was added to sync old tenants to have a tenant name
	// This should be deleted on next release
	TypeTenantRefreshName: {
		Enabled:  ptr.PointTo(true),
		Cronspec: "0 3 * * *", // At 03:00 AM daily
		Retries:  ptr.PointTo(defaultRetryCount),
	},
}
