package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/OpenCHAMI/magellan/internal/server"
	// Register vendor plugins so the service's manager can dispatch (and to
	// exercise the same wiring the daemon uses in production).
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/OpenCHAMI/magellan/pkg/service"
	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
	"github.com/stmcginnis/gofish/schemas"
)

func systemJSON(state schemas.PowerState) string {
	return fmt.Sprintf(`{
		"@odata.id": "/redfish/v1/Systems/Node0",
		"@odata.type": "#ComputerSystem.v1_5_0.ComputerSystem",
		"Id": "Node0", "Name": "Node0", "PowerState": "%s",
		"Actions": { "#ComputerSystem.Reset": {
			"ResetType@Redfish.AllowableValues": ["On","ForceOff","GracefulShutdown","ForceRestart"],
			"target": "/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
		}}
	}`, state)
}

// newMockRedfish stands up a stateful single-system Redfish service whose power
// state reacts to reset actions.
func newMockRedfish(t *testing.T, initial schemas.PowerState) *httptest.Server {
	t.Helper()
	var (
		mu    sync.Mutex
		state = initial
	)
	mux := chi.NewMux()
	mux.HandleFunc("/redfish/v1/", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1/Systems", test.Make(test.RESPONSE_Systems))
	mux.HandleFunc("/redfish/v1/Systems/Node0", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		s := state
		mu.Unlock()
		_, _ = w.Write([]byte(systemJSON(s)))
	})
	mux.HandleFunc("/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ ResetType string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		switch schemas.ResetType(b.ResetType) {
		case schemas.OnResetType, schemas.ForceOnResetType:
			state = schemas.OnPowerState
		case schemas.ForceOffResetType, schemas.GracefulShutdownResetType:
			state = schemas.OffPowerState
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestHandler(t *testing.T, cfg server.Config) http.Handler {
	t.Helper()
	svc := service.New(secrets.NewStaticStore("test", "test"))
	svc.Insecure = true
	t.Cleanup(svc.Close)
	return server.New(svc, cfg).Handler()
}

// do issues an in-process request against the handler.
func do(t *testing.T, h http.Handler, method, target string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req = httptest.NewRequest(method, target, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return m
}

func powerQuery(base, bmcURI, system string) string {
	q := url.Values{}
	q.Set("bmc", bmcURI)
	q.Set("system", system)
	return base + "?" + q.Encode()
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	rec := do(t, h, http.MethodGet, "/healthz", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", rec.Code)
	}
	if got := decodeBody(t, rec)["status"]; got != "ok" {
		t.Fatalf("/healthz status field = %v, want ok", got)
	}
}

func TestPowerStateEndpoint(t *testing.T) {
	mock := newMockRedfish(t, schemas.OnPowerState)
	h := newTestHandler(t, server.Config{})

	rec := do(t, h, http.MethodGet, powerQuery("/v1/power", mock.URL, "Node0"), nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["powerState"]; got != string(schemas.OnPowerState) {
		t.Fatalf("powerState = %v, want On", got)
	}
}

func TestPowerStateMissingParams(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	rec := do(t, h, http.MethodGet, "/v1/power?bmc=https://x", nil, nil) // no system
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestResetTypesEndpoint(t *testing.T) {
	mock := newMockRedfish(t, schemas.OnPowerState)
	h := newTestHandler(t, server.Config{})

	rec := do(t, h, http.MethodGet, powerQuery("/v1/power/reset-types", mock.URL, "Node0"), nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	types, ok := decodeBody(t, rec)["resetTypes"].([]any)
	if !ok || len(types) == 0 {
		t.Fatalf("resetTypes = %v, want a non-empty list", decodeBody(t, rec)["resetTypes"])
	}
}

func TestPowerActionOperationNoWait(t *testing.T) {
	mock := newMockRedfish(t, schemas.OffPowerState)
	h := newTestHandler(t, server.Config{})

	rec := do(t, h, http.MethodPost, "/v1/power",
		map[string]any{"bmc": mock.URL, "system": "Node0", "operation": "on"}, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["issued"]; got != true {
		t.Fatalf("issued = %v, want true", got)
	}
}

func TestPowerActionOperationWaitConfirmed(t *testing.T) {
	// Node starts Off; "on" with wait should drive it On and confirm.
	mock := newMockRedfish(t, schemas.OffPowerState)
	h := newTestHandler(t, server.Config{})

	rec := do(t, h, http.MethodPost, "/v1/power",
		map[string]any{"bmc": mock.URL, "system": "Node0", "operation": "on", "wait": true}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (confirmed); body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["status"] != "confirmed" {
		t.Fatalf("status = %v, want confirmed", body["status"])
	}
	if body["finalState"] != string(schemas.OnPowerState) {
		t.Fatalf("finalState = %v, want On", body["finalState"])
	}
}

func TestPowerActionUnknownOperation(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	rec := do(t, h, http.MethodPost, "/v1/power",
		map[string]any{"bmc": "https://x", "system": "Node0", "operation": "teleport"}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestPowerActionWaitWithRawResetRejected(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	rec := do(t, h, http.MethodPost, "/v1/power",
		map[string]any{"bmc": "https://x", "system": "Node0", "resetType": "On", "wait": true}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPowerActionRequiresExactlyOneAction(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	// Neither operation nor resetType.
	rec := do(t, h, http.MethodPost, "/v1/power",
		map[string]any{"bmc": "https://x", "system": "Node0"}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestInventoryMissingBMC(t *testing.T) {
	h := newTestHandler(t, server.Config{})
	rec := do(t, h, http.MethodPost, "/v1/inventory", map[string]any{}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestBearerAuth(t *testing.T) {
	const token = "s3cret-token"
	h := newTestHandler(t, server.Config{AuthToken: token})

	// Health is unauthenticated.
	if rec := do(t, h, http.MethodGet, "/healthz", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("/healthz with auth on = %d, want 200", rec.Code)
	}
	// /v1 without a token is rejected.
	if rec := do(t, h, http.MethodGet, "/v1/power?bmc=https://x&system=Node0", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-token /v1 = %d, want 401", rec.Code)
	}
	// Wrong token is rejected.
	if rec := do(t, h, http.MethodGet, "/v1/power?bmc=https://x&system=Node0", nil,
		map[string]string{"Authorization": "Bearer wrong"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-token /v1 = %d, want 401", rec.Code)
	}
	// Correct token passes auth (then fails downstream on the bogus BMC, which is
	// a 502 — proving it got past the auth gate).
	if rec := do(t, h, http.MethodGet, "/v1/power?bmc=https://x&system=Node0", nil,
		map[string]string{"Authorization": "Bearer " + token}); rec.Code == http.StatusUnauthorized {
		t.Fatalf("correct-token /v1 = 401, want it to pass the auth gate")
	}
}
