package constants

// needed for using own type to avoid collisions
type internalUserDataKey string
type businessUserDataKey string
type userTypeKey string
type UserTypeValue string

const (
	UserType     userTypeKey   = "UserType"
	BusinessUser UserTypeValue = "Business"
	InternalUser UserTypeValue = "Internal"

	InternalUserData internalUserDataKey = "InternalData"
	BusinessUserData businessUserDataKey = "ClientData"
)
