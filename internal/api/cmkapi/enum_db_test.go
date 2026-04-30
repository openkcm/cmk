package cmkapi_test

import (
	"errors"
	"testing"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

// --- KeyState tests ---

func TestKeyState_Valid(t *testing.T) {
	if !cmkapi.KeyStateENABLED.Valid() {
		t.Fatal("expected ENABLED to be valid")
	}
	if cmkapi.KeyState("BOGUS").Valid() {
		t.Fatal("expected BOGUS to be invalid")
	}
}

func TestKeyState_Value_Valid(t *testing.T) {
	v, err := cmkapi.KeyStateENABLED.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "ENABLED" {
		t.Fatalf("expected ENABLED, got %v", v)
	}
}

func TestKeyState_Value_Empty(t *testing.T) {
	v, err := cmkapi.KeyState("").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty, got %v", v)
	}
}

func TestKeyState_Value_Invalid(t *testing.T) {
	_, err := cmkapi.KeyState("BOGUS").Value()
	if !errors.Is(err, cmkapi.ErrInvalidKeyState) {
		t.Fatalf("expected ErrInvalidKeyState, got %v", err)
	}
}

func TestKeyState_Scan_String(t *testing.T) {
	var s cmkapi.KeyState
	if err := s.Scan("ENABLED"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != cmkapi.KeyStateENABLED {
		t.Fatalf("expected ENABLED, got %s", s)
	}
}

func TestKeyState_Scan_Bytes(t *testing.T) {
	var s cmkapi.KeyState
	if err := s.Scan([]byte("DISABLED")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != cmkapi.KeyStateDISABLED {
		t.Fatalf("expected DISABLED, got %s", s)
	}
}

func TestKeyState_Scan_Nil(t *testing.T) {
	var s cmkapi.KeyState
	if err := s.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty, got %s", s)
	}
}

func TestKeyState_Scan_Invalid(t *testing.T) {
	var s cmkapi.KeyState
	err := s.Scan("BOGUS")
	if !errors.Is(err, cmkapi.ErrInvalidKeyState) {
		t.Fatalf("expected ErrInvalidKeyState, got %v", err)
	}
}

func TestKeyState_Scan_WrongType(t *testing.T) {
	var s cmkapi.KeyState
	err := s.Scan(123)
	if !errors.Is(err, cmkapi.ErrUnexpectedScanType) {
		t.Fatalf("expected ErrUnexpectedScanType, got %v", err)
	}
}

// --- SystemStatus tests ---

func TestSystemStatus_Valid(t *testing.T) {
	if !cmkapi.SystemStatusCONNECTED.Valid() {
		t.Fatal("expected CONNECTED to be valid")
	}
	if cmkapi.SystemStatus("BOGUS").Valid() {
		t.Fatal("expected BOGUS to be invalid")
	}
}

func TestSystemStatus_Value_Valid(t *testing.T) {
	v, err := cmkapi.SystemStatusCONNECTED.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "CONNECTED" {
		t.Fatalf("expected CONNECTED, got %v", v)
	}
}

func TestSystemStatus_Value_Empty(t *testing.T) {
	v, err := cmkapi.SystemStatus("").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty, got %v", v)
	}
}

func TestSystemStatus_Value_Invalid(t *testing.T) {
	_, err := cmkapi.SystemStatus("BOGUS").Value()
	if !errors.Is(err, cmkapi.ErrInvalidSystemStatus) {
		t.Fatalf("expected ErrInvalidSystemStatus, got %v", err)
	}
}

func TestSystemStatus_Scan_String(t *testing.T) {
	var s cmkapi.SystemStatus
	if err := s.Scan("CONNECTED"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != cmkapi.SystemStatusCONNECTED {
		t.Fatalf("expected CONNECTED, got %s", s)
	}
}

func TestSystemStatus_Scan_Bytes(t *testing.T) {
	var s cmkapi.SystemStatus
	if err := s.Scan([]byte("FAILED")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != cmkapi.SystemStatusFAILED {
		t.Fatalf("expected FAILED, got %s", s)
	}
}

func TestSystemStatus_Scan_Nil(t *testing.T) {
	var s cmkapi.SystemStatus
	if err := s.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty, got %s", s)
	}
}

func TestSystemStatus_Scan_Invalid(t *testing.T) {
	var s cmkapi.SystemStatus
	err := s.Scan("BOGUS")
	if !errors.Is(err, cmkapi.ErrInvalidSystemStatus) {
		t.Fatalf("expected ErrInvalidSystemStatus, got %v", err)
	}
}

func TestSystemStatus_Scan_WrongType(t *testing.T) {
	var s cmkapi.SystemStatus
	err := s.Scan(99)
	if !errors.Is(err, cmkapi.ErrUnexpectedScanType) {
		t.Fatalf("expected ErrUnexpectedScanType, got %v", err)
	}
}
