package main

var (
	EnabledKeyStatus       = "ENABLED"
	DisabledKeyStatus      = "DISABLED"
	CreatedKeyStatus       = "CREATED"
	PendingImportKeyStatus = "PENDING_IMPORT"
	UnknownKeyStatus       = "UNKNOWN"
)

type KeyRecord struct {
	KeyID  string `gorm:"primaryKey;column:key_id"`
	Status string
}
