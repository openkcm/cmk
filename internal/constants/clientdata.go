package constants

// needed for using own type to avoid collisions
type internalDataKey string
type clientDataKey string
type sourceKey string
type SourceValue string

const (
	Source         sourceKey       = "Source"
	BusinessSource SourceValue     = "Business"
	InternalSource SourceValue     = "Internal"
	InternalData   internalDataKey = "InternalData"
	ClientData     clientDataKey   = "ClientData"
)
