package bmc

import (
	"testing"

	"github.com/stmcginnis/gofish"
)

// serviceWithVendor builds a minimal gofish client whose ServiceRoot reports the
// given vendor string, without touching the network.
func serviceWithVendor(vendor string) *gofish.APIClient {
	return &gofish.APIClient{Service: &gofish.Service{Vendor: vendor}}
}

func TestClientForMatchingDetector(t *testing.T) {
	// Register a sentinel plugin that only matches a vendor string no real
	// plugin (registered via the vendors blank-import in this test binary) or
	// other test would ever produce, so it cannot interfere with other cases.
	const sentinel = "ZZZ-TEST-VENDOR"
	RegisterVendor("zzz-test", func(api *gofish.APIClient) bool {
		return api != nil && api.Service != nil && api.Service.Vendor == sentinel
	}, func(api *gofish.APIClient) Client {
		return NewGenericClient(api, "zzz-test")
	})

	got := clientFor(serviceWithVendor(sentinel))
	if got.Vendor() != "zzz-test" {
		t.Fatalf("clientFor selected vendor %q, want the registered sentinel plugin %q", got.Vendor(), "zzz-test")
	}
}

func TestClientForFallsBackToGeneric(t *testing.T) {
	// "ACME" matches no registered detector, so clientFor must fall back to a
	// GenericClient carrying the ServiceRoot vendor string verbatim.
	got := clientFor(serviceWithVendor("ACME"))
	if got.Vendor() != Vendor("ACME") {
		t.Fatalf("clientFor fallback vendor = %q, want %q", got.Vendor(), "ACME")
	}
	if _, ok := got.(*GenericClient); !ok {
		t.Fatalf("clientFor fallback returned %T, want *GenericClient", got)
	}
}

func TestDetectVendorString(t *testing.T) {
	cases := []struct {
		name string
		api  *gofish.APIClient
		want Vendor
	}{
		{"nil client", nil, VendorGeneric},
		{"nil service", &gofish.APIClient{}, VendorGeneric},
		{"empty vendor", &gofish.APIClient{Service: &gofish.Service{}}, VendorGeneric},
		{"populated vendor", serviceWithVendor("Dell Inc."), Vendor("Dell Inc.")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectVendorString(tc.api); got != tc.want {
				t.Fatalf("detectVendorString = %q, want %q", got, tc.want)
			}
		})
	}
}
