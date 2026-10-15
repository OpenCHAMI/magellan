package bmc

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
)

// Detector reports whether a connected BMC is handled by a particular vendor
// plugin, based on its Redfish ServiceRoot metadata.
type Detector func(api *gofish.APIClient) bool

// Factory builds a vendor-specific Client around a connected gofish client.
type Factory func(api *gofish.APIClient) Client

type vendorPlugin struct {
	detect  Detector
	factory Factory
}

var (
	registryMu sync.RWMutex
	registry   = make(map[Vendor]vendorPlugin)
)

// RegisterVendor registers a vendor plugin's detector and factory. Vendor
// plugins call this from their init() functions; the aggregator package
// (pkg/bmc/vendors) is blank-imported to trigger registration.
func RegisterVendor(vendor Vendor, detect Detector, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[vendor] = vendorPlugin{detect: detect, factory: factory}
}

// clientFor wraps a connected gofish client in a vendor Client. When forced is
// empty, it auto-detects the vendor by running each registered plugin's
// Detector against the ServiceRoot, falling back to a GenericClient tagged with
// the ServiceRoot Vendor string when no plugin matches. When forced is
// non-empty, it returns exactly the requested client (generic or a specific
// plugin) and errors on an unknown vendor rather than silently falling back.
func clientFor(api *gofish.APIClient, forced Vendor) (Client, error) {
	if forced != "" {
		return forcedClient(api, forced)
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	for vendor, plugin := range registry {
		if plugin.detect != nil && plugin.detect(api) {
			log.Debug().Str("vendor", string(vendor)).Msg("matched vendor plugin for BMC")
			return plugin.factory(api), nil
		}
	}
	return NewGenericClient(api, detectVendorString(api)), nil
}

// forcedClient returns the Client for an explicitly requested vendor. The
// generic vendor bypasses the plugin registry; any other value must name a
// registered plugin or it is rejected.
func forcedClient(api *gofish.APIClient, forced Vendor) (Client, error) {
	if forced == VendorGeneric {
		log.Debug().Msg("using generic Redfish client (forced)")
		return NewGenericClient(api, VendorGeneric), nil
	}
	registryMu.RLock()
	plugin, ok := registry[forced]
	registryMu.RUnlock()
	if !ok || plugin.factory == nil {
		return nil, fmt.Errorf("unknown vendor %q (valid: %s)", forced, validVendorList())
	}
	log.Debug().Str("vendor", string(forced)).Msg("using forced vendor plugin for BMC")
	return plugin.factory(api), nil
}

// ValidateVendor reports whether forced names a usable client selection: empty
// (auto-detect), the generic fallback, or a registered plugin. It returns an
// error for any other value so callers can reject a typo before connecting.
func ValidateVendor(forced Vendor) error {
	if forced == "" || forced == VendorGeneric {
		return nil
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	if _, ok := registry[forced]; !ok {
		return fmt.Errorf("unknown vendor %q (valid: %s)", forced, validVendorList())
	}
	return nil
}

// ParseVendor parses a case-insensitive vendor name (e.g. from a CLI flag) into
// a validated Vendor. It returns an error for unknown names.
func ParseVendor(s string) (Vendor, error) {
	v := Vendor(strings.ToLower(strings.TrimSpace(s)))
	if err := ValidateVendor(v); err != nil {
		return "", err
	}
	return v, nil
}

// RegisteredVendors returns the generic fallback followed by every vendor with a
// registered plugin, sorted for stable output.
func RegisteredVendors() []Vendor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	plugins := make([]Vendor, 0, len(registry))
	for v := range registry {
		plugins = append(plugins, v)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i] < plugins[j] })
	vendors := make([]Vendor, 0, len(plugins)+1)
	vendors = append(vendors, VendorGeneric)
	return append(vendors, plugins...)
}

// validVendorList renders the selectable vendors as a comma-separated string
// for error messages.
func validVendorList() string {
	vendors := RegisteredVendors()
	parts := make([]string, len(vendors))
	for i, v := range vendors {
		parts[i] = string(v)
	}
	return strings.Join(parts, ", ")
}

// detectVendorString returns the vendor reported by the Redfish ServiceRoot, or
// VendorGeneric when it is unavailable.
func detectVendorString(api *gofish.APIClient) Vendor {
	if api == nil || api.Service == nil || api.Service.Vendor == "" {
		return VendorGeneric
	}
	return Vendor(api.Service.Vendor)
}
