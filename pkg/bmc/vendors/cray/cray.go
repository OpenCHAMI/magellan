// Package cray provides Cray/HPE-Cray-specific BMC handling. For now it
// registers a detector and a Client that embeds the generic Redfish client;
// Cray-specific quirks are implemented here as the abstraction grows.
package cray

import (
	"strings"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish"
)

func init() {
	bmc.RegisterVendor(bmc.VendorCray, detect, newClient)
}

// detect reports whether the BMC's Redfish ServiceRoot identifies it as Cray.
func detect(api *gofish.APIClient) bool {
	if api == nil || api.Service == nil {
		return false
	}
	return strings.Contains(strings.ToLower(api.Service.Vendor), "cray")
}

func newClient(api *gofish.APIClient) bmc.Client {
	return &Client{GenericClient: bmc.NewGenericClient(api, bmc.VendorCray)}
}

// Client adds Cray-specific quirk handling on top of the generic Redfish client.
type Client struct {
	*bmc.GenericClient
}
