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

	got, err := clientFor(serviceWithVendor(sentinel), "")
	if err != nil {
		t.Fatalf("clientFor returned error: %v", err)
	}
	if got.Vendor() != "zzz-test" {
		t.Fatalf("clientFor selected vendor %q, want the registered sentinel plugin %q", got.Vendor(), "zzz-test")
	}
}

func TestClientForFallsBackToGeneric(t *testing.T) {
	// "ACME" matches no registered detector, so clientFor must fall back to a
	// GenericClient carrying the ServiceRoot vendor string verbatim.
	got, err := clientFor(serviceWithVendor("ACME"), "")
	if err != nil {
		t.Fatalf("clientFor returned error: %v", err)
	}
	if got.Vendor() != Vendor("ACME") {
		t.Fatalf("clientFor fallback vendor = %q, want %q", got.Vendor(), "ACME")
	}
	if _, ok := got.(*GenericClient); !ok {
		t.Fatalf("clientFor fallback returned %T, want *GenericClient", got)
	}
}

func TestClientForForcedVendor(t *testing.T) {
	// Register a sentinel plugin so we can force it regardless of detection.
	RegisterVendor("zzz-test", func(api *gofish.APIClient) bool {
		return false // never auto-matches; only reachable by forcing
	}, func(api *gofish.APIClient) Client {
		return NewGenericClient(api, "zzz-test")
	})

	// Forcing a registered vendor bypasses detection and returns that plugin,
	// even though the ServiceRoot reports a different vendor.
	got, err := clientFor(serviceWithVendor("Dell Inc."), "zzz-test")
	if err != nil {
		t.Fatalf("clientFor(forced) returned error: %v", err)
	}
	if got.Vendor() != "zzz-test" {
		t.Fatalf("clientFor(forced) vendor = %q, want %q", got.Vendor(), "zzz-test")
	}

	// Forcing "generic" returns a generic client regardless of ServiceRoot.
	got, err = clientFor(serviceWithVendor("HPE"), VendorGeneric)
	if err != nil {
		t.Fatalf("clientFor(forced generic) returned error: %v", err)
	}
	if _, ok := got.(*GenericClient); !ok {
		t.Fatalf("clientFor(forced generic) returned %T, want *GenericClient", got)
	}

	// Forcing an unknown vendor is an error, not a silent generic fallback.
	if _, err := clientFor(serviceWithVendor("Dell Inc."), "no-such-vendor"); err == nil {
		t.Fatal("clientFor(forced unknown) = nil error, want an error")
	}
}

func TestValidateAndParseVendor(t *testing.T) {
	// Empty and generic are always valid.
	for _, v := range []Vendor{"", VendorGeneric} {
		if err := ValidateVendor(v); err != nil {
			t.Fatalf("ValidateVendor(%q) = %v, want nil", v, err)
		}
	}
	// A registered plugin vendor is valid.
	if err := ValidateVendor("hpe"); err != nil {
		t.Fatalf("ValidateVendor(hpe) = %v, want nil", err)
	}
	// An unknown vendor is invalid.
	if err := ValidateVendor("no-such-vendor"); err == nil {
		t.Fatal("ValidateVendor(unknown) = nil error, want an error")
	}

	// ParseVendor lowercases and trims.
	got, err := ParseVendor("  HPE  ")
	if err != nil {
		t.Fatalf("ParseVendor(HPE) = %v, want nil", err)
	}
	if got != "hpe" {
		t.Fatalf("ParseVendor(HPE) = %q, want %q", got, "hpe")
	}
	// ParseVendor rejects unknown names.
	if _, err := ParseVendor("nope"); err == nil {
		t.Fatal("ParseVendor(unknown) = nil error, want an error")
	}

	// RegisteredVendors always includes generic and is non-empty.
	vendors := RegisteredVendors()
	if len(vendors) == 0 || vendors[0] != VendorGeneric {
		t.Fatalf("RegisteredVendors() = %v, want generic first", vendors)
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
