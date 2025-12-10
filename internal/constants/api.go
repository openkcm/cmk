package constants

const (
	APIName  = "CMK"
	KMS      = "KMS"
	BasePath = "/cmk/v1/{tenant}"
)

const (
	KeyTypeSystemManaged = "SYSTEM_MANAGED"
	KeyTypeBYOK          = "BYOK"
	KeyTypeHYOK          = "HYOK"

	DefaultKeyStore = "DEFAULT_KEYSTORE"
	HYOKKeyStore    = "HYOK_KEYSTORE"
)

const (
	DefaultTop  = 20
	DefaultSkip = 0
)
