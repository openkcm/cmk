package cmkapi

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrInvalidKeyState is returned when a key state value is not recognized.
	ErrInvalidKeyState = errors.New("invalid key state")
	// ErrInvalidSystemStatus is returned when a system status value is not recognized.
	ErrInvalidSystemStatus = errors.New("invalid system status")
	// ErrUnexpectedScanType is returned when Scan receives a value that is not string or []byte.
	ErrUnexpectedScanType = errors.New("unexpected scan type")
)

// Valid KeyState values.
var validKeyStates = []KeyState{
	KeyStateDELETED, KeyStateDETACHED, KeyStateDETACHING, KeyStateDISABLED,
	KeyStateENABLED, KeyStateFORBIDDEN, KeyStatePENDINGDELETION,
	KeyStatePENDINGIMPORT, KeyStateUNKNOWN,
}

// Valid SystemStatus values.
var validSystemStatuses = []SystemStatus{
	SystemStatusCONNECTED, SystemStatusDISCONNECTED,
	SystemStatusFAILED, SystemStatusPROCESSING,
}

// Valid returns true if the KeyState is a known value.
func (s KeyState) Valid() bool {
	return slices.Contains(validKeyStates, s)
}

// Value implements the driver.Valuer interface.
func (s KeyState) Value() (driver.Value, error) {
	if s == "" {
		return "", nil
	}
	if !s.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidKeyState, s)
	}
	return string(s), nil
}

// Scan implements the sql.Scanner interface.
func (s *KeyState) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}
	var sv string
	switch v := value.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, value)
	}
	*s = KeyState(sv)
	if !s.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidKeyState, sv)
	}
	return nil
}

// Valid returns true if the SystemStatus is a known value.
func (s SystemStatus) Valid() bool {
	return slices.Contains(validSystemStatuses, s)
}

// Value implements the driver.Valuer interface.
func (s SystemStatus) Value() (driver.Value, error) {
	if s == "" {
		return "", nil
	}
	if !s.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidSystemStatus, s)
	}
	return string(s), nil
}

// Scan implements the sql.Scanner interface.
func (s *SystemStatus) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}
	var sv string
	switch v := value.(type) {
	case string:
		sv = v
	case []byte:
		sv = string(v)
	default:
		return fmt.Errorf("%w: expected string or []byte, got %T", ErrUnexpectedScanType, value)
	}
	*s = SystemStatus(sv)
	if !s.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidSystemStatus, sv)
	}
	return nil
}
