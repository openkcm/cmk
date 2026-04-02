package constants

import "github.com/google/uuid"

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

// SystemUser Do not add further internal users without blocklisting in the clientdata
var SystemUser uuid.UUID = uuid.Max
