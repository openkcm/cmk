package config

const (
	TypeSystemsTask     = "sys:refresh"
	TypeCertificateTask = "cert:rotate"
	TypeHYOKSync        = "key:sync"
	TypeKeystorePool    = "keystore:fill"
)

var DefinedTasks = map[string]struct{}{
	TypeSystemsTask:     {},
	TypeCertificateTask: {},
	TypeHYOKSync:        {},
	TypeKeystorePool:    {},
}
