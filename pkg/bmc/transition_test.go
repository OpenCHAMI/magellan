package bmc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// powerMock is a stateful in-memory Redfish ComputerSystem whose power state
// changes in response to reset actions, letting transition tests drive the
// confirm / timeout / escalation paths deterministically.
type powerMock struct {
	mu             sync.Mutex
	state          schemas.PowerState
	resets         []string // reset types received, in order
	ignoreGraceful bool     // model a node that ignores GracefulShutdown
}

func (pm *powerMock) systemJSON() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return fmt.Sprintf(`{
		"@odata.id": "/redfish/v1/Systems/Node0",
		"@odata.type": "#ComputerSystem.v1_5_0.ComputerSystem",
		"Id": "Node0",
		"Name": "Node0",
		"PowerState": "%s",
		"Actions": {
			"#ComputerSystem.Reset": {
				"ResetType@Redfish.AllowableValues": ["On","ForceOff","GracefulShutdown","ForceRestart","PowerCycle"],
				"target": "/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
			}
		}
	}`, pm.state)
}

func (pm *powerMock) applyReset(rt schemas.ResetType) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.resets = append(pm.resets, string(rt))
	switch rt {
	case schemas.OnResetType, schemas.ForceOnResetType:
		pm.state = schemas.OnPowerState
	case schemas.ForceOffResetType:
		pm.state = schemas.OffPowerState
	case schemas.GracefulShutdownResetType:
		if !pm.ignoreGraceful {
			pm.state = schemas.OffPowerState
		}
	case schemas.GracefulRestartResetType, schemas.ForceRestartResetType, schemas.PowerCycleResetType:
		// A restart ends in the On state, as it began.
		pm.state = schemas.OnPowerState
	}
}

func (pm *powerMock) resetLog() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return append([]string(nil), pm.resets...)
}

// newPowerMock stands up the stateful Redfish service and returns a connected
// GenericClient plus the mock for assertions.
func newPowerMock(t *testing.T, initial schemas.PowerState, ignoreGraceful bool) (*GenericClient, *powerMock) {
	t.Helper()
	pm := &powerMock{state: initial, ignoreGraceful: ignoreGraceful}

	mux := chi.NewMux()
	mux.HandleFunc("/redfish/v1/", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1/Systems", test.Make(test.RESPONSE_Systems))
	mux.HandleFunc("/redfish/v1/Systems/Node0", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(pm.systemJSON()))
	})
	mux.HandleFunc("/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ ResetType string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		pm.applyReset(schemas.ResetType(body.ResetType))
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	api, err := gofish.Connect(gofish.ClientConfig{
		Endpoint: srv.URL, Username: "test", Password: "test", Insecure: true, BasicAuth: true,
	})
	if err != nil {
		t.Fatalf("failed to connect to mock Redfish service: %v", err)
	}
	t.Cleanup(api.Logout)
	return NewGenericClient(api, VendorGeneric), pm
}

// fastOpts confirms quickly so timeout-driven tests stay sub-second.
func fastOpts() TransitionOptions {
	return TransitionOptions{
		PollInterval: 2 * time.Millisecond,
		Timeout:      80 * time.Millisecond,
		Retries:      0,
		Escalate:     true,
	}
}

func TestResetAndConfirm_OnConfirmed(t *testing.T) {
	gc, _ := newPowerMock(t, schemas.OffPowerState, false)

	res, err := ResetAndConfirm(context.Background(), gc, "Node0", OpOn, fastOpts())
	if err != nil {
		t.Fatalf("ResetAndConfirm(on) error: %v", err)
	}
	if !res.Confirmed() {
		t.Fatalf("status = %q, want confirmed", res.Status)
	}
	if res.FinalState != schemas.OnPowerState {
		t.Fatalf("final state = %q, want On", res.FinalState)
	}
	if res.Escalated {
		t.Fatal("unexpected escalation on a successful 'on'")
	}
}

func TestResetAndConfirm_TimeoutNoEscalation(t *testing.T) {
	// soft-off is graceful-only and must NOT escalate; a node that ignores
	// GracefulShutdown therefore times out unconfirmed.
	gc, pm := newPowerMock(t, schemas.OnPowerState, true)

	res, err := ResetAndConfirm(context.Background(), gc, "Node0", OpSoftOff, fastOpts())
	if err != nil {
		t.Fatalf("ResetAndConfirm(soft-off) error: %v", err)
	}
	if res.Status != StatusTimedOut {
		t.Fatalf("status = %q, want timed-out", res.Status)
	}
	if res.Escalated {
		t.Fatal("soft-off must not escalate")
	}
	if got := pm.resetLog(); len(got) != 1 || got[0] != string(schemas.GracefulShutdownResetType) {
		t.Fatalf("reset log = %v, want a single GracefulShutdown", got)
	}
}

func TestResetAndConfirm_GracefulEscalatesToForce(t *testing.T) {
	// A node ignoring GracefulShutdown: "off" must time out gracefully, then
	// escalate to ForceOff, which the node honors → confirmed.
	gc, pm := newPowerMock(t, schemas.OnPowerState, true)

	res, err := ResetAndConfirm(context.Background(), gc, "Node0", OpOff, fastOpts())
	if err != nil {
		t.Fatalf("ResetAndConfirm(off) error: %v", err)
	}
	if !res.Confirmed() {
		t.Fatalf("status = %q, want confirmed after escalation", res.Status)
	}
	if !res.Escalated || res.EscalatedTo != OpForceOff {
		t.Fatalf("escalated=%v to=%q, want true to force-off", res.Escalated, res.EscalatedTo)
	}
	if res.FinalState != schemas.OffPowerState {
		t.Fatalf("final state = %q, want Off", res.FinalState)
	}
	want := []string{string(schemas.GracefulShutdownResetType), string(schemas.ForceOffResetType)}
	if got := pm.resetLog(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reset log = %v, want %v", got, want)
	}
}

func TestResetAndConfirm_RestartUnconfirmable(t *testing.T) {
	// A restart has no stable power-state target, and the mock returns no async
	// task, so the operation is issued but reported unconfirmable.
	gc, _ := newPowerMock(t, schemas.OnPowerState, false)

	res, err := ResetAndConfirm(context.Background(), gc, "Node0", OpHardRestart, fastOpts())
	if err != nil {
		t.Fatalf("ResetAndConfirm(hard-restart) error: %v", err)
	}
	if res.Status != StatusUnconfirmable {
		t.Fatalf("status = %q, want unconfirmable", res.Status)
	}
}

func TestResetAndConfirm_ContextCancelled(t *testing.T) {
	gc, _ := newPowerMock(t, schemas.OnPowerState, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := ResetAndConfirm(ctx, gc, "Node0", OpOff, fastOpts())
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res == nil {
		t.Fatal("expected a non-nil result even on a failed issue")
	}
}

func TestTransitionOptionsDefaults(t *testing.T) {
	// Zero PollInterval/Timeout are defaulted, but Retries == 0 is a valid value
	// (one attempt, no retry) and must be preserved; only Retries < 0 defaults.
	o := TransitionOptions{}.withDefaults()
	if o.PollInterval != DefaultPollInterval || o.Timeout != DefaultTimeout {
		t.Fatalf("withDefaults() = %+v, want interval/timeout defaulted", o)
	}
	if o.Retries != 0 {
		t.Fatalf("withDefaults() Retries = %d, want 0 preserved", o.Retries)
	}
	if neg := (TransitionOptions{Retries: -1}).withDefaults(); neg.Retries != DefaultPollRetries {
		t.Fatalf("withDefaults() negative Retries = %d, want %d", neg.Retries, DefaultPollRetries)
	}
	if d := DefaultTransitionOptions(); !d.Escalate {
		t.Fatal("DefaultTransitionOptions().Escalate = false, want true")
	}
}
