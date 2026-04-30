package workflow_test

import (
	"errors"
	"testing"

	"github.com/openkcm/cmk/internal/workflow"
)

// --- State tests ---

func TestState_Valid(t *testing.T) {
	if !workflow.StateInitial.Valid() {
		t.Fatal("expected INITIAL to be valid")
	}
	if workflow.State("BOGUS").Valid() {
		t.Fatal("expected BOGUS to be invalid")
	}
}

func TestState_Value_Valid(t *testing.T) {
	v, err := workflow.StateInitial.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "INITIAL" {
		t.Fatalf("expected INITIAL, got %v", v)
	}
}

func TestState_Value_Empty(t *testing.T) {
	v, err := workflow.State("").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty, got %v", v)
	}
}

func TestState_Value_Invalid(t *testing.T) {
	_, err := workflow.State("BOGUS").Value()
	if !errors.Is(err, workflow.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
}

func TestState_Scan_String(t *testing.T) {
	var s workflow.State
	if err := s.Scan("INITIAL"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != workflow.StateInitial {
		t.Fatalf("expected INITIAL, got %s", s)
	}
}

func TestState_Scan_Bytes(t *testing.T) {
	var s workflow.State
	if err := s.Scan([]byte("WAIT_APPROVAL")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != workflow.StateWaitApproval {
		t.Fatalf("expected WAIT_APPROVAL, got %s", s)
	}
}

func TestState_Scan_Nil(t *testing.T) {
	var s workflow.State
	if err := s.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty, got %s", s)
	}
}

func TestState_Scan_Invalid(t *testing.T) {
	var s workflow.State
	err := s.Scan("BOGUS")
	if !errors.Is(err, workflow.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
}

func TestState_Scan_WrongType(t *testing.T) {
	var s workflow.State
	err := s.Scan(123)
	if !errors.Is(err, workflow.ErrUnexpectedScanType) {
		t.Fatalf("expected ErrUnexpectedScanType, got %v", err)
	}
}

// --- ActionType tests ---

func TestActionType_Valid(t *testing.T) {
	if !workflow.ActionTypeDelete.Valid() {
		t.Fatal("expected DELETE to be valid")
	}
	if workflow.ActionType("BOGUS").Valid() {
		t.Fatal("expected BOGUS to be invalid")
	}
}

func TestActionType_Value_Valid(t *testing.T) {
	v, err := workflow.ActionTypeDelete.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "DELETE" {
		t.Fatalf("expected DELETE, got %v", v)
	}
}

func TestActionType_Value_Empty(t *testing.T) {
	v, err := workflow.ActionType("").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty, got %v", v)
	}
}

func TestActionType_Value_Invalid(t *testing.T) {
	_, err := workflow.ActionType("BOGUS").Value()
	if !errors.Is(err, workflow.ErrInvalidActionType) {
		t.Fatalf("expected ErrInvalidActionType, got %v", err)
	}
}

func TestActionType_Scan_String(t *testing.T) {
	var a workflow.ActionType
	if err := a.Scan("DELETE"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != workflow.ActionTypeDelete {
		t.Fatalf("expected DELETE, got %s", a)
	}
}

func TestActionType_Scan_Bytes(t *testing.T) {
	var a workflow.ActionType
	if err := a.Scan([]byte("LINK")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != workflow.ActionTypeLink {
		t.Fatalf("expected LINK, got %s", a)
	}
}

func TestActionType_Scan_Nil(t *testing.T) {
	var a workflow.ActionType
	if err := a.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != "" {
		t.Fatalf("expected empty, got %s", a)
	}
}

func TestActionType_Scan_Invalid(t *testing.T) {
	var a workflow.ActionType
	err := a.Scan("INVALID")
	if !errors.Is(err, workflow.ErrInvalidActionType) {
		t.Fatalf("expected ErrInvalidActionType, got %v", err)
	}
}

func TestActionType_Scan_WrongType(t *testing.T) {
	var a workflow.ActionType
	err := a.Scan(42)
	if !errors.Is(err, workflow.ErrUnexpectedScanType) {
		t.Fatalf("expected ErrUnexpectedScanType, got %v", err)
	}
}

// --- ArtifactType tests ---

func TestArtifactType_Valid(t *testing.T) {
	if !workflow.ArtifactTypeKey.Valid() {
		t.Fatal("expected KEY to be valid")
	}
	if workflow.ArtifactType("BOGUS").Valid() {
		t.Fatal("expected BOGUS to be invalid")
	}
}

func TestArtifactType_Value_Valid(t *testing.T) {
	v, err := workflow.ArtifactTypeKey.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "KEY" {
		t.Fatalf("expected KEY, got %v", v)
	}
}

func TestArtifactType_Value_Empty(t *testing.T) {
	v, err := workflow.ArtifactType("").Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("expected empty, got %v", v)
	}
}

func TestArtifactType_Value_Invalid(t *testing.T) {
	_, err := workflow.ArtifactType("BOGUS").Value()
	if !errors.Is(err, workflow.ErrInvalidArtifactType) {
		t.Fatalf("expected ErrInvalidArtifactType, got %v", err)
	}
}

func TestArtifactType_Scan_String(t *testing.T) {
	var a workflow.ArtifactType
	if err := a.Scan("KEY"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != workflow.ArtifactTypeKey {
		t.Fatalf("expected KEY, got %s", a)
	}
}

func TestArtifactType_Scan_Bytes(t *testing.T) {
	var a workflow.ArtifactType
	if err := a.Scan([]byte("SYSTEM")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != workflow.ArtifactTypeSystem {
		t.Fatalf("expected SYSTEM, got %s", a)
	}
}

func TestArtifactType_Scan_Nil(t *testing.T) {
	var a workflow.ArtifactType
	if err := a.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != "" {
		t.Fatalf("expected empty, got %s", a)
	}
}

func TestArtifactType_Scan_Invalid(t *testing.T) {
	var a workflow.ArtifactType
	err := a.Scan("INVALID")
	if !errors.Is(err, workflow.ErrInvalidArtifactType) {
		t.Fatalf("expected ErrInvalidArtifactType, got %v", err)
	}
}

func TestArtifactType_Scan_WrongType(t *testing.T) {
	var a workflow.ArtifactType
	err := a.Scan(3.14)
	if !errors.Is(err, workflow.ErrUnexpectedScanType) {
		t.Fatalf("expected ErrUnexpectedScanType, got %v", err)
	}
}
