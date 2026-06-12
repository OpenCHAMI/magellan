package bmc

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
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
	state, err := c.GetPowerState("Node0")
	if err != nil {
		t.Fatalf("GetPowerState(Node0) unexpected error: %v", err)
	}
	if state != redfish.OnPowerState {
		t.Fatalf("GetPowerState(Node0) = %q, want %q", state, redfish.OnPowerState)
	}
}

func TestGenericClientResetTypesFound(t *testing.T) {
	c := newMockGenericClient(t)
	got, err := c.GetResetTypes("Node0")
	if err != nil {
		t.Fatalf("GetResetTypes(Node0) unexpected error: %v", err)
	}
	want := map[redfish.ResetType]bool{
		redfish.OnResetType:               true,
		redfish.ForceOffResetType:         true,
		redfish.GracefulShutdownResetType: true,
		redfish.ForceRestartResetType:     true,
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

	if _, err := c.GetPowerState("bogus"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetPowerState(bogus) err = %v, want a 'not found' error", err)
	}
	if _, err := c.GetResetTypes("bogus"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetResetTypes(bogus) err = %v, want a 'not found' error", err)
	}
	if err := c.Reset("bogus", redfish.OnResetType); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Reset(bogus) err = %v, want a 'not found' error", err)
	}
}

func TestNewGenericClientDefaultsVendor(t *testing.T) {
	if v := NewGenericClient(nil, "").Vendor(); v != VendorGeneric {
		t.Fatalf("NewGenericClient empty vendor = %q, want %q", v, VendorGeneric)
	}
}
