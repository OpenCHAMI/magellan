package bmc

import (
	"fmt"
	"sort"

	"github.com/stmcginnis/gofish/schemas"
)

// Operation is a vendor-neutral power operation. Callers request an Operation
// (e.g. "off") and the BMC layer resolves it to a concrete Redfish ResetType the
// target actually advertises, applying a fallback chain. This shields callers
// from having to know which exact Redfish token a given BMC accepts — the gap
// called out in bugs.md power-parity #2.
type Operation string

const (
	// OpOn powers the target on.
	OpOn Operation = "on"
	// OpOff powers the target off gracefully, falling back to a forced power-off.
	OpOff Operation = "off"
	// OpSoftOff requests a graceful shutdown only, with no forced fallback.
	OpSoftOff Operation = "soft-off"
	// OpForceOff powers the target off immediately, without a graceful attempt.
	OpForceOff Operation = "force-off"
	// OpSoftRestart restarts gracefully, falling back to a forced restart.
	OpSoftRestart Operation = "soft-restart"
	// OpHardRestart forces a restart, falling back to a power cycle.
	OpHardRestart Operation = "hard-restart"
	// OpInit power-cycles the target, falling back to a forced restart.
	OpInit Operation = "init"
)

// operationPreferences maps each Operation to an ordered list of Redfish reset
// types. ResolveResetType picks the first entry the target advertises as
// supported. The ordering encodes the graceful→forced fallback policy mirrored
// from power-control (PCS): e.g. "off" prefers GracefulShutdown but accepts
// ForceOff, while "soft-off" intentionally has no forced fallback.
var operationPreferences = map[Operation][]schemas.ResetType{
	OpOn:          {schemas.OnResetType, schemas.ForceOnResetType},
	OpOff:         {schemas.GracefulShutdownResetType, schemas.ForceOffResetType},
	OpSoftOff:     {schemas.GracefulShutdownResetType},
	OpForceOff:    {schemas.ForceOffResetType},
	OpSoftRestart: {schemas.GracefulRestartResetType, schemas.ForceRestartResetType},
	OpHardRestart: {schemas.ForceRestartResetType, schemas.PowerCycleResetType},
	OpInit:        {schemas.PowerCycleResetType, schemas.ForceRestartResetType},
}

// ErrUnsupportedOperation reports that a known Operation could not be mapped to
// any reset type the target advertises. It is deliberately distinct from a BMC
// call failure so callers can report "unsupported" separately from "failed"
// (bugs.md power-parity #2).
type ErrUnsupportedOperation struct {
	Op        Operation
	Supported []schemas.ResetType
}

func (e *ErrUnsupportedOperation) Error() string {
	return fmt.Sprintf("power operation %q is not supported by this system; advertised reset types: %v",
		e.Op, e.Supported)
}

// KnownOperation reports whether op is a recognized power operation.
func KnownOperation(op Operation) bool {
	_, ok := operationPreferences[op]
	return ok
}

// Operations returns the known power operations in sorted order, for help text
// and input validation.
func Operations() []Operation {
	ops := make([]Operation, 0, len(operationPreferences))
	for op := range operationPreferences {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i] < ops[j] })
	return ops
}

// ResolveResetType maps a vendor-neutral Operation to a concrete Redfish
// ResetType given the set the target advertises as supported.
//
//   - If supported is non-empty, the operation's preference list is honored
//     strictly: the first preferred reset type that appears in supported wins,
//     and if none match, ErrUnsupportedOperation is returned.
//   - If supported is empty (the BMC advertised no reset types), the preferred
//     reset type is returned on a best-effort basis. This mirrors gofish's own
//     Reset, which assumes the request is acceptable when the system advertises
//     no AllowableValues.
//   - An unrecognized Operation yields an error.
func ResolveResetType(op Operation, supported []schemas.ResetType) (schemas.ResetType, error) {
	prefs, ok := operationPreferences[op]
	if !ok {
		return "", fmt.Errorf("unknown power operation %q (known: %v)", op, Operations())
	}

	if len(supported) == 0 {
		// Nothing advertised; best-effort with the preferred reset type.
		return prefs[0], nil
	}

	supportedSet := make(map[schemas.ResetType]bool, len(supported))
	for _, s := range supported {
		supportedSet[s] = true
	}
	for _, p := range prefs {
		if supportedSet[p] {
			return p, nil
		}
	}
	return "", &ErrUnsupportedOperation{Op: op, Supported: supported}
}
