package bmc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// newMockGenericClient stands up an in-memory Redfish service exposing a single
// ComputerSystem (Node0) and returns a GenericClient connected to it. The server
// is torn down automatically when the test finishes.
func newMockGenericClient(t *testing.T) *GenericClient {
	t.Helper()
	mux := chi.NewMux()
	// gofish requests the service root at "/redfish/v1/"; register the
	// no-trailing-slash form too for safety.
	mux.HandleFunc("/redfish/v1/", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1/Systems", test.Make(test.RESPONSE_Systems))
	mux.HandleFunc("/redfish/v1/Systems/Node0", test.Make(test.RESPONSE_System_Node0))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	api, err := gofish.Connect(gofish.ClientConfig{
		Endpoint:  srv.URL,
		Username:  "test",
		Password:  "test",
		Insecure:  true,
		BasicAuth: true,
	})
	if err != nil {
		t.Fatalf("failed to connect to mock Redfish service: %v", err)
	}
	t.Cleanup(api.Logout)
	return NewGenericClient(api, VendorGeneric)
}

func TestGenericClientPowerStateFound(t *testing.T) {
	c := newMockGenericClient(t)
	state, err := c.GetPowerState(context.Background(), "Node0")
	if err != nil {
		t.Fatalf("GetPowerState(Node0) unexpected error: %v", err)
	}
	if state != schemas.OnPowerState {
		t.Fatalf("GetPowerState(Node0) = %q, want %q", state, schemas.OnPowerState)
	}
}

func TestGenericClientResetTypesFound(t *testing.T) {
	c := newMockGenericClient(t)
	got, err := c.GetResetTypes(context.Background(), "Node0")
	if err != nil {
		t.Fatalf("GetResetTypes(Node0) unexpected error: %v", err)
	}
	want := map[schemas.ResetType]bool{
		schemas.OnResetType:               true,
		schemas.ForceOffResetType:         true,
		schemas.GracefulShutdownResetType: true,
		schemas.ForceRestartResetType:     true,
	}
	if len(got) != len(want) {
		t.Fatalf("GetResetTypes(Node0) = %v, want %d types", got, len(want))
	}
	for _, rt := range got {
		if !want[rt] {
			t.Fatalf("GetResetTypes(Node0) returned unexpected type %q", rt)
		}
	}
}

// TestGenericClientSystemNotFound guards the deliberate "return an error, do not
// panic" behavior of systemByID for an unknown system ID. The pre-refactor code
// dereferenced a nil system here.
func TestGenericClientSystemNotFound(t *testing.T) {
	c := newMockGenericClient(t)
	ctx := context.Background()

	if _, err := c.GetPowerState(ctx, "bogus"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetPowerState(bogus) err = %v, want a 'not found' error", err)
	}
	if _, err := c.GetResetTypes(ctx, "bogus"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetResetTypes(bogus) err = %v, want a 'not found' error", err)
	}
	if _, err := c.Reset(ctx, "bogus", schemas.OnResetType); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Reset(bogus) err = %v, want a 'not found' error", err)
	}
	if _, err := c.ResetOperation(ctx, "bogus", OpOff); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ResetOperation(bogus) err = %v, want a 'not found' error", err)
	}
}

// TestGenericClientContextCancelled verifies ctx acts as an entry guard: a
// cancelled context aborts before any BMC I/O, returning the context error.
func TestGenericClientContextCancelled(t *testing.T) {
	c := newMockGenericClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.GetPowerState(ctx, "Node0"); err != context.Canceled {
		t.Fatalf("GetPowerState with cancelled ctx err = %v, want context.Canceled", err)
	}
}

// TestGenericClientResetOperationResolvesAndPosts is the end-to-end check that a
// vendor-neutral operation is resolved against the system's advertised reset
// types and the resolved type is what actually gets POSTed. Node0 advertises
// GracefulShutdown, so "off" must resolve to GracefulShutdown (not ForceOff).
func TestGenericClientResetOperationResolvesAndPosts(t *testing.T) {
	var gotBody string
	mux := chi.NewMux()
	mux.HandleFunc("/redfish/v1/", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1", test.Make(test.RESPONSE_ServiceRoot))
	mux.HandleFunc("/redfish/v1/Systems", test.Make(test.RESPONSE_Systems))
	mux.HandleFunc("/redfish/v1/Systems/Node0", test.Make(test.RESPONSE_System_Node0))
	mux.HandleFunc("/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
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
	c := NewGenericClient(api, VendorGeneric)

	if _, err := c.ResetOperation(context.Background(), "Node0", OpOff); err != nil {
		t.Fatalf("ResetOperation(off) unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, string(schemas.GracefulShutdownResetType)) {
		t.Fatalf("reset POST body = %q, want it to contain %q", gotBody, schemas.GracefulShutdownResetType)
	}
}

func TestNewGenericClientDefaultsVendor(t *testing.T) {
	if v := NewGenericClient(nil, "").Vendor(); v != VendorGeneric {
		t.Fatalf("NewGenericClient empty vendor = %q, want %q", v, VendorGeneric)
	}
}
