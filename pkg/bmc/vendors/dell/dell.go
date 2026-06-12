// Package dell provides Dell/iDRAC-specific BMC handling. For now it registers a
// detector and a Client that embeds the generic Redfish client; Dell-specific
// quirks (firmware UpdateService actions, BIOS Oem.Dell namespace, ETag handling
// on power operations) are implemented here as the abstraction grows.
package dell

import (
	"strings"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish"
)

func init() {
	bmc.RegisterVendor(bmc.VendorDell, detect, newClient)
}

// detect reports whether the BMC's Redfish ServiceRoot identifies it as Dell.
func detect(api *gofish.APIClient) bool {
	if api == nil || api.Service == nil {
		return false
	}
	return strings.Contains(strings.ToLower(api.Service.Vendor), "dell")
}

func newClient(api *gofish.APIClient) bmc.Client {
	return &Client{GenericClient: bmc.NewGenericClient(api, bmc.VendorDell)}
}

// Client adds Dell-specific quirk handling on top of the generic Redfish client.
type Client struct {
	*bmc.GenericClient
}
