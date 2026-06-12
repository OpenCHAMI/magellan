package bmc_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	// Blank-import the vendor plugins so their init() registrations run; this is
	// what wires up vendor detection for the dispatch test below. Living in the
	// external bmc_test package avoids the import cycle a same-package test would
	// hit (vendors imports bmc).
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/OpenCHAMI/magellan/pkg/test"
	"github.com/go-chi/chi/v5"
)

// mockServer stands up a Redfish service root serving the given document and
// returns a ConnConfig pointed at it with working static credentials.
func mockServer(t *testing.T, serviceRoot string) bmc.ConnConfig {
	t.Helper()
	mux := chi.NewMux()
	mux.HandleFunc("/redfish/v1/", test.Make(serviceRoot))
	mux.HandleFunc("/redfish/v1", test.Make(serviceRoot))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return bmc.ConnConfig{
		URI:             srv.URL,
		Insecure:        true,
		CredentialStore: secrets.NewStaticStore("test", "test"),
		UseDefault:      true,
	}
}

func TestManagerCachedClientReusesSession(t *testing.T) {
	cfg := mockServer(t, test.RESPONSE_ServiceRoot)
	m := bmc.NewManager()

	c1, err := m.CachedClient(cfg)
	if err != nil {
		t.Fatalf("CachedClient: %v", err)
	}
	c2, err := m.CachedClient(cfg)
	if err != nil {
		t.Fatalf("CachedClient (2nd): %v", err)
	}
	if c1 != c2 {
		t.Fatal("CachedClient returned a different client for the same URI; cache miss")
	}

	// LogoutAll must evict, so the next CachedClient builds a fresh session.
	m.LogoutAll()
	c3, err := m.CachedClient(cfg)
	if err != nil {
		t.Fatalf("CachedClient after LogoutAll: %v", err)
	}
	if c1 == c3 {
		t.Fatal("CachedClient returned the evicted client after LogoutAll")
	}
}

func TestManagerClientIsUncached(t *testing.T) {
	cfg := mockServer(t, test.RESPONSE_ServiceRoot)
	m := bmc.NewManager()

	c1, err := m.Client(cfg)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	c2, err := m.Client(cfg)
	if err != nil {
		t.Fatalf("Client (2nd): %v", err)
	}
	if c1 == c2 {
		t.Fatal("Client returned the same client twice; it must not cache")
	}
}

// TestManagerDispatchesToVendorPlugin proves the full init()-registration →
// detect → factory chain: a ServiceRoot advertising "HPE" must be served by the
// HPE plugin, not the generic fallback.
func TestManagerDispatchesToVendorPlugin(t *testing.T) {
	cfg := mockServer(t, test.RESPONSE_ServiceRoot_HPE)
	c, err := bmc.NewManager().Client(cfg)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if c.Vendor() != bmc.VendorHPE {
		t.Fatalf("dispatched client Vendor = %q, want %q", c.Vendor(), bmc.VendorHPE)
	}
}

func TestManagerConnectDecoratesErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{"404 not a BMC", http.StatusNotFound, "no ServiceRoot found"},
		{"401 auth failed", http.StatusUnauthorized, "authentication failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := chi.NewMux()
			mux.HandleFunc("/redfish/v1/", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", tc.status)
			})
			mux.HandleFunc("/redfish/v1", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", tc.status)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			cfg := bmc.ConnConfig{URI: srv.URL, Insecure: true, CredentialStore: secrets.NewStaticStore("test", "test")}
			_, err := bmc.NewManager().Connect(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Connect err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestManagerConnectNilStoreFailsFast(t *testing.T) {
	// No server: a nil credential store must fail during credential resolution,
	// before any network call is attempted.
	cfg := bmc.ConnConfig{URI: "https://unreachable.invalid", CredentialStore: nil}
	_, err := bmc.NewManager().Connect(cfg)
	if err == nil || !strings.Contains(err.Error(), "credential store is invalid") {
		t.Fatalf("Connect with nil store err = %v, want 'credential store is invalid'", err)
	}
}
