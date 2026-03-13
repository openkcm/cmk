package constants

const (
	APIName  = "CMK"
	KMS      = "KMS"
	BasePath = "/cmk/v1/{tenant}"
)

const (
	KeyTypeBYOK = "BYOK"
	KeyTypeHYOK = "HYOK"

	DefaultKeyStore = "DEFAULT_KEYSTORE"
	HYOKKeyStore    = "HYOK_KEYSTORE"
)

const (
	DefaultTop  = 20
	DefaultSkip = 0
)

const (
	QueryMaxLengthSystem = 50
	QueryMaxLengthName   = 255
)
