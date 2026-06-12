// Package hpe provides HPE/iLO-specific BMC handling. For now it registers a
// detector and a Client that embeds the generic Redfish client; HPE-specific
// quirks (firmware UpdateService actions, BIOS Oem.Hpe namespace, manager
// metadata layout) are implemented here as the abstraction grows.
package hpe

import (
	"strings"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish"
)

func init() {
	bmc.RegisterVendor(bmc.VendorHPE, detect, newClient)
}

// detect reports whether the BMC's Redfish ServiceRoot identifies it as HPE.
func detect(api *gofish.APIClient) bool {
	if api == nil || api.Service == nil {
		return false
	}
	vendor := strings.ToLower(api.Service.Vendor)
	return strings.Contains(vendor, "hpe") || strings.Contains(vendor, "hewlett")
}

func newClient(api *gofish.APIClient) bmc.Client {
	return &Client{GenericClient: bmc.NewGenericClient(api, bmc.VendorHPE)}
}

// Client adds HPE-specific quirk handling on top of the generic Redfish client.
type Client struct {
	*bmc.GenericClient
}
