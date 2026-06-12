package bmc

import (
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

// clientFor wraps a connected gofish client in the most specific registered
// vendor Client whose detector matches, falling back to a GenericClient using
// the ServiceRoot Vendor string when no plugin matches.
func clientFor(api *gofish.APIClient) Client {
	registryMu.RLock()
	defer registryMu.RUnlock()
	for vendor, plugin := range registry {
		if plugin.detect != nil && plugin.detect(api) {
			log.Debug().Str("vendor", string(vendor)).Msg("matched vendor plugin for BMC")
			return plugin.factory(api)
		}
	}
	return NewGenericClient(api, detectVendorString(api))
}

// detectVendorString returns the vendor reported by the Redfish ServiceRoot, or
// VendorGeneric when it is unavailable.
func detectVendorString(api *gofish.APIClient) Vendor {
	if api == nil || api.Service == nil || api.Service.Vendor == "" {
		return VendorGeneric
	}
	return Vendor(api.Service.Vendor)
}
