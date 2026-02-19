package eventprocessor

type JobType string

const (
	JobTypeSystemLink            JobType = "SYSTEM_LINK"
	JobTypeSystemUnlink          JobType = "SYSTEM_UNLINK"
	JobTypeSystemSwitch          JobType = "SYSTEM_SWITCH"
	JobTypeKeyEnable             JobType = "KEY_ENABLE"
	JobTypeKeyDisable            JobType = "KEY_DISABLE"
	JobTypeKeyConfigDecommission JobType = "KEY_CONFIG_DECOMMISSION"
)

// KeyActionJobData contains the data needed for a key action orbital job.
type KeyActionJobData struct {
	TenantID string `json:"tenantID"`
	KeyID    string `json:"keyID"`
}

// SystemActionJobData contains the data needed for a system action orbital job.
type SystemActionJobData struct {
	SystemID  string `json:"systemID"`
	TenantID  string `json:"tenantID"`
	KeyIDTo   string `json:"keyIDTo"`
	KeyIDFrom string `json:"keyIDFrom"`
	Trigger   string `json:"trigger,omitempty"`
}

// KeyConfigActionJobData contains the data needed for a key configuration action orbital job.
type KeyConfigActionJobData struct {
	TenantID    string `json:"tenantID"`
	KeyConfigID string `json:"keyConfigID"`
}
