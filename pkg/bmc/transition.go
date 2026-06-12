package bmc

import (
	"context"
	"time"

	"github.com/stmcginnis/gofish/schemas"
)

// Defaults for power-transition confirmation. They mirror power-control (PCS):
// poll roughly every 15s against a 5-minute deadline, retrying transient read
// failures a few times.
const (
	DefaultPollInterval = 15 * time.Second
	DefaultTimeout      = 5 * time.Minute
	DefaultPollRetries  = 3
)

// TransitionOptions configures how ResetAndConfirm confirms that a power
// operation took effect.
type TransitionOptions struct {
	// PollInterval is the delay between power-state polls. Defaults to
	// DefaultPollInterval when <= 0.
	PollInterval time.Duration
	// Timeout bounds the confirmation of a single operation. Defaults to
	// DefaultTimeout when <= 0.
	Timeout time.Duration
	// Retries is the number of additional attempts for each power-state poll
	// (reads are idempotent and safe to retry). Defaults to DefaultPollRetries
	// when < 0. The reset action itself is issued exactly once to avoid
	// duplicate power operations.
	Retries int
	// Escalate enables the graceful→forced fallback: when a graceful operation
	// (e.g. "off") fails to reach its target before the deadline, the forced
	// equivalent ("force-off") is issued and confirmed.
	Escalate bool
}

// DefaultTransitionOptions returns options with all defaults applied and
// escalation enabled. Callers should start here and override as needed, because
// the zero value of Escalate (false) disables the graceful→forced fallback.
func DefaultTransitionOptions() TransitionOptions {
	return TransitionOptions{
		PollInterval: DefaultPollInterval,
		Timeout:      DefaultTimeout,
		Retries:      DefaultPollRetries,
		Escalate:     true,
	}
}

func (o TransitionOptions) withDefaults() TransitionOptions {
	if o.PollInterval <= 0 {
		o.PollInterval = DefaultPollInterval
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Retries < 0 {
		o.Retries = DefaultPollRetries
	}
	return o
}

// TransitionStatus is the outcome of confirming a power operation.
type TransitionStatus string

const (
	// StatusConfirmed means the target reached its expected power state, or the
	// BMC's async task completed successfully.
	StatusConfirmed TransitionStatus = "confirmed"
	// StatusTimedOut means the reset was issued but the target did not reach its
	// expected power state before the deadline.
	StatusTimedOut TransitionStatus = "timed-out"
	// StatusUnconfirmable means the reset was issued but cannot be confirmed via
	// power state (e.g. a restart, whose terminal state is indistinguishable
	// from its starting state) and the BMC supplied no trackable task.
	StatusUnconfirmable TransitionStatus = "unconfirmable"
)

// TransitionResult describes the outcome of a ResetAndConfirm call.
type TransitionResult struct {
	// Operation is the operation originally requested.
	Operation Operation
	// Status is the confirmation outcome.
	Status TransitionStatus
	// FinalState is the last observed power state (empty if never read).
	FinalState schemas.PowerState
	// Escalated reports whether a forced fallback was issued after a graceful
	// operation timed out.
	Escalated bool
	// EscalatedTo is the forced operation issued when Escalated is true.
	EscalatedTo Operation
	// Task is the most recent gofish task-monitor handle, when the BMC modeled
	// the reset asynchronously (may be nil).
	Task *schemas.TaskMonitorInfo
}

// Confirmed reports whether the transition reached its target state.
func (r *TransitionResult) Confirmed() bool { return r.Status == StatusConfirmed }

// targetPowerState returns the power state an operation is expected to settle
// into, and whether such a stable target exists. Restarts have no stable
// power-state target (they end On, as they began), so they return false.
func targetPowerState(op Operation) (schemas.PowerState, bool) {
	switch op {
	case OpOn:
		return schemas.OnPowerState, true
	case OpOff, OpSoftOff, OpForceOff:
		return schemas.OffPowerState, true
	default:
		return "", false
	}
}

// forcedEscalation returns the forced operation to fall back to when a graceful
// operation times out, and whether escalation applies. "soft-off" deliberately
// does not escalate — it is the caller's explicit "graceful only" request.
func forcedEscalation(op Operation) (Operation, bool) {
	switch op {
	case OpOff:
		return OpForceOff, true
	case OpSoftRestart:
		return OpHardRestart, true
	default:
		return "", false
	}
}

// ResetAndConfirm performs a vendor-neutral power Operation and confirms it took
// effect. It issues the operation (resolving it to a supported reset type), then
// confirms completion by following the BMC's async task to a terminal state when
// one is returned, otherwise by polling the target's power state until it matches
// the expected state or the deadline elapses. When a graceful operation times
// out and opts.Escalate is set, the forced equivalent is issued and confirmed.
//
// It returns a non-nil TransitionResult describing the outcome even when the
// operation could not be confirmed (Status reflects that); a non-nil error is
// returned only when the operation could not be issued at all (e.g. an
// unsupported operation or a connection failure).
func ResetAndConfirm(ctx context.Context, c Client, systemID string, op Operation, opts TransitionOptions) (*TransitionResult, error) {
	opts = opts.withDefaults()
	result := &TransitionResult{Operation: op}

	task, err := c.ResetOperation(ctx, systemID, op)
	if err != nil {
		return result, err
	}
	result.Task = task

	status, state := confirm(ctx, c, systemID, op, task, opts)
	result.Status = status
	result.FinalState = state
	if status != StatusTimedOut {
		return result, nil
	}

	// Graceful operation did not reach its target in time; escalate if asked.
	if opts.Escalate {
		if escOp, ok := forcedEscalation(op); ok {
			result.Escalated = true
			result.EscalatedTo = escOp

			task2, err := c.ResetOperation(ctx, systemID, escOp)
			if err != nil {
				return result, err
			}
			result.Task = task2

			status2, state2 := confirm(ctx, c, systemID, escOp, task2, opts)
			result.Status = status2
			result.FinalState = state2
		}
	}
	return result, nil
}

// confirm waits for an issued operation to take effect, returning the outcome
// and the last observed power state. It bounds itself to opts.Timeout.
func confirm(ctx context.Context, c Client, systemID string, op Operation, task *schemas.TaskMonitorInfo, opts TransitionOptions) (TransitionStatus, schemas.PowerState) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	target, hasTarget := targetPowerState(op)

	// Native path: if the BMC modeled the reset as an async task, follow it to
	// completion. On success, verify against the target state when one exists.
	if task != nil && task.TaskMonitor != "" {
		if _, err := schemas.WaitForTaskMonitor(ctx, c.Gofish(), opts.PollInterval, task, nil); err == nil {
			if !hasTarget {
				return StatusConfirmed, ""
			}
			if state, err := pollPowerState(ctx, c, systemID, opts.Retries); err == nil && state == target {
				return StatusConfirmed, state
			}
			// Task completed but state does not match yet; fall through to polling.
		}
		// Task wait failed; fall through to power-state polling where possible.
	}

	if !hasTarget {
		// A restart with no usable task signal: issued, but not confirmable via
		// power state alone.
		return StatusUnconfirmable, ""
	}

	// Poll power state until it matches the target or the deadline elapses.
	var last schemas.PowerState
	for {
		if state, err := pollPowerState(ctx, c, systemID, opts.Retries); err == nil {
			last = state
			if state == target {
				return StatusConfirmed, state
			}
		}

		timer := time.NewTimer(opts.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return StatusTimedOut, last
		case <-timer.C:
		}
	}
}

// pollPowerState reads the power state once, retrying transient failures up to
// retries additional times. It honors ctx between attempts.
func pollPowerState(ctx context.Context, c Client, systemID string, retries int) (schemas.PowerState, error) {
	var (
		state schemas.PowerState
		err   error
	)
	for attempt := 0; attempt <= retries; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return state, cerr
		}
		if state, err = c.GetPowerState(ctx, systemID); err == nil {
			return state, nil
		}
	}
	return state, err
}
