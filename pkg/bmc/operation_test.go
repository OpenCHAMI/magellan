package bmc

import (
	"errors"
	"testing"

	"github.com/stmcginnis/gofish/schemas"
)

func TestResolveResetType(t *testing.T) {
	// A typical advertised set that supports both graceful and forced variants.
	full := []schemas.ResetType{
		schemas.OnResetType,
		schemas.ForceOnResetType,
		schemas.GracefulShutdownResetType,
		schemas.ForceOffResetType,
		schemas.GracefulRestartResetType,
		schemas.ForceRestartResetType,
		schemas.PowerCycleResetType,
	}

	cases := []struct {
		name      string
		op        Operation
		supported []schemas.ResetType
		want      schemas.ResetType
	}{
		{"on prefers On", OpOn, full, schemas.OnResetType},
		{"off prefers graceful", OpOff, full, schemas.GracefulShutdownResetType},
		{"soft-restart prefers graceful", OpSoftRestart, full, schemas.GracefulRestartResetType},
		{"hard-restart prefers force", OpHardRestart, full, schemas.ForceRestartResetType},
		{"force-off is force-off", OpForceOff, full, schemas.ForceOffResetType},
		{"init prefers power cycle", OpInit, full, schemas.PowerCycleResetType},

		// Fallbacks: graceful variant absent, forced variant present.
		{"off falls back to ForceOff", OpOff,
			[]schemas.ResetType{schemas.ForceOffResetType}, schemas.ForceOffResetType},
		{"soft-restart falls back to ForceRestart", OpSoftRestart,
			[]schemas.ResetType{schemas.ForceRestartResetType}, schemas.ForceRestartResetType},
		{"hard-restart falls back to PowerCycle", OpHardRestart,
			[]schemas.ResetType{schemas.PowerCycleResetType}, schemas.PowerCycleResetType},

		// Empty advertised set → best-effort preferred type.
		{"empty supported → preferred", OpOff, nil, schemas.GracefulShutdownResetType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveResetType(tc.op, tc.supported)
			if err != nil {
				t.Fatalf("ResolveResetType(%q) unexpected error: %v", tc.op, err)
			}
			if got != tc.want {
				t.Fatalf("ResolveResetType(%q) = %q, want %q", tc.op, got, tc.want)
			}
		})
	}
}

// TestResolveResetTypeUnsupported verifies that when the advertised set excludes
// every preference for an operation, the resolver reports it as unsupported (a
// typed error distinct from a BMC call failure) rather than guessing.
func TestResolveResetTypeUnsupported(t *testing.T) {
	// soft-off only accepts GracefulShutdown; a BMC advertising only ForceOff
	// cannot satisfy it.
	_, err := ResolveResetType(OpSoftOff, []schemas.ResetType{schemas.ForceOffResetType})
	if err == nil {
		t.Fatal("expected ErrUnsupportedOperation, got nil")
	}
	var unsupported *ErrUnsupportedOperation
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v (%T), want *ErrUnsupportedOperation", err, err)
	}
	if unsupported.Op != OpSoftOff {
		t.Fatalf("unsupported.Op = %q, want %q", unsupported.Op, OpSoftOff)
	}
}

func TestResolveResetTypeUnknownOperation(t *testing.T) {
	_, err := ResolveResetType(Operation("teleport"), nil)
	if err == nil {
		t.Fatal("expected error for unknown operation, got nil")
	}
	// An unknown operation is a caller error, not an ErrUnsupportedOperation.
	var unsupported *ErrUnsupportedOperation
	if errors.As(err, &unsupported) {
		t.Fatalf("unknown operation should not be ErrUnsupportedOperation, got %v", err)
	}
}

func TestKnownOperationAndOperations(t *testing.T) {
	if !KnownOperation(OpOn) {
		t.Fatal("KnownOperation(OpOn) = false, want true")
	}
	if KnownOperation(Operation("nope")) {
		t.Fatal("KnownOperation(nope) = true, want false")
	}
	// Operations() must list every operation that has a resolver mapping, sorted.
	ops := Operations()
	if len(ops) != len(operationPreferences) {
		t.Fatalf("Operations() len = %d, want %d", len(ops), len(operationPreferences))
	}
	for i := 1; i < len(ops); i++ {
		if ops[i-1] >= ops[i] {
			t.Fatalf("Operations() not sorted: %v", ops)
		}
	}
}
