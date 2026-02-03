package constants

import "github.com/google/uuid"

// needed for using own type to avoid collisions
type clientDataKey string

type ContextClientData struct {
	Identifier string
	Groups     []string
}

const (
	ClientData clientDataKey = "ClientData"
)

// SystemUser Do not add further internal users without blocklisting in the clientdata
var SystemUser uuid.UUID = uuid.Max
