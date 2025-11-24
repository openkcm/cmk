package config

const (
	TypeSystemsTask        = "sys:refresh"
	TypeCertificateTask    = "cert:rotate"
	TypeHYOKSync           = "key:sync"
	TypeKeystorePool       = "keystore:fill"
	TypeSendNotifications  = "notify:send"
	TypeWorkflowAutoAssign = "workflow:auto-assign"
)

var DefinedTasks = map[string]struct{}{
	TypeSystemsTask:        {},
	TypeCertificateTask:    {},
	TypeHYOKSync:           {},
	TypeKeystorePool:       {},
	TypeSendNotifications:  {},
	TypeWorkflowAutoAssign: {},
}
