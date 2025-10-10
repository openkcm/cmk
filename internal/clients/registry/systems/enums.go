package systems

import (
	"errors"
	"strings"
)

// SystemType represents the type of the system.
type SystemType string

// Defines values for SystemType.
const (
	SystemTypeSYSTEM     SystemType = "SYSTEM"
	SystemTypeSUBACCOUNT SystemType = "SUBACCOUNT"
)

var ErrUnsupportSystemType = errors.New("unsupported system type ")

func systemTypeStringMap(responseType string) (*SystemType, error) {
	stringMap := map[string]SystemType{
		"SYSTEM":     SystemTypeSYSTEM,
		"SUBACCOUNT": SystemTypeSUBACCOUNT,
	}

	systemType, ok := stringMap[strings.ToUpper(responseType)]
	if !ok {
		return nil, ErrUnsupportSystemType
	}

	return &systemType, nil
}
