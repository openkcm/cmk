package systems

import (
	"errors"
	"strings"

	"github.com/openkcm/cmk/internal/model"
)

// SystemType aliases model.SystemType — the canonical definition (and
// persisted Valuer/Scanner) lives in internal/model.
type SystemType = model.SystemType

const (
	SystemTypeSYSTEM     = model.SystemTypeSYSTEM
	SystemTypeSUBACCOUNT = model.SystemTypeSUBACCOUNT
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
