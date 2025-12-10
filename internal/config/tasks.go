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

var DefinedTasks = map[string]struct{}{
	TypeSystemsTask:        {},
	TypeCertificateTask:    {},
	TypeHYOKSync:           {},
	TypeKeystorePool:       {},
	TypeSendNotifications:  {},
	TypeWorkflowAutoAssign: {},
	TypeWorkflowCleanup:    {},
	TypeWorkflowExpire:     {},
}
