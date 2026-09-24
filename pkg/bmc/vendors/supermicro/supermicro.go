// Package supermicro provides Supermicro-specific BMC handling. For now it
// registers a detector and a Client that embeds the generic Redfish client;
// Supermicro-specific quirks (BIOS Oem.Supermicro namespace, firmware update
// shapes) are implemented here as the abstraction grows.
package supermicro

import (
	"strings"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish"
)

func init() {
	bmc.RegisterVendor(bmc.VendorSupermicro, detect, newClient)
}

// detect reports whether the BMC's Redfish ServiceRoot identifies it as Supermicro.
func detect(api *gofish.APIClient) bool {
	if api == nil || api.Service == nil {
		return false
	}
	vendor := strings.ToLower(api.Service.Vendor)
	return strings.Contains(vendor, "supermicro") || strings.Contains(vendor, "super micro")
}

func newClient(api *gofish.APIClient) bmc.Client {
	return &Client{GenericClient: bmc.NewGenericClient(api, bmc.VendorSupermicro)}
}

// Client adds Supermicro-specific quirk handling on top of the generic Redfish client.
type Client struct {
	*bmc.GenericClient
}
