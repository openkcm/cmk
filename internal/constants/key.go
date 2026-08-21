package constants

const (
	KeyActionSetPrimary      = "SET_PRIMARY_KEY"
	SystemActionDecommission = "SYSTEM_DECOMMISSION"

	// KeyTypeBYOK mirrors cmkapi.KeyType BYOK, defined here to avoid import cycles.
	KeyTypeBYOK = "BYOK"
	// KeyTypeHYOK mirrors cmkapi.KeyType HYOK, defined here to avoid import cycles.
	KeyTypeHYOK = "HYOK"
	// KeyTypeSystemManaged mirrors cmkapi.KeyType SYSTEM_MANAGED, defined here to avoid import cycles.
	KeyTypeSystemManaged = "SYSTEM_MANAGED"
)
