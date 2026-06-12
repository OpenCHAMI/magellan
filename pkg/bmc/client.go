package bmc

import (
	"fmt"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// Vendor identifies the manufacturer of a BMC. It is used to dispatch to
// vendor-specific quirk handling. VendorGeneric is the fallback used for
// unknown vendors and for vendors that have no registered plugin.
type Vendor string

const (
	VendorGeneric    Vendor = "generic"
	VendorHPE        Vendor = "hpe"
	VendorDell       Vendor = "dell"
	VendorSupermicro Vendor = "supermicro"
	VendorCray       Vendor = "cray"
	VendorLenovo     Vendor = "lenovo"
)

// Client is the vendor-shielded interface that callers use to operate a BMC.
// Implementations encapsulate vendor-specific behavior so that callers never
// need to special-case a manufacturer. The generic implementation
// (GenericClient) wraps gofish directly and is used as the fallback for unknown
// vendors; vendor plugins under pkg/bmc/vendors embed it and override only the
// operations that differ.
//
// Gofish exposes the underlying gofish client for operations that have not yet
// been hoisted behind the abstraction (e.g. inventory crawling). As the
// abstraction grows, callers should prefer the typed methods over Gofish.
type Client interface {
	// Gofish returns the underlying gofish API client.
	Gofish() *gofish.APIClient
	// Vendor returns the detected vendor for this BMC.
	Vendor() Vendor
	// Logout terminates the underlying BMC session.
	Logout()

	// GetPowerState returns the power state of the ComputerSystem with the
	// given Redfish ID.
	GetPowerState(systemID string) (schemas.PowerState, error)
	// GetResetTypes returns the reset types supported by the ComputerSystem
	// with the given Redfish ID.
	GetResetTypes(systemID string) ([]schemas.ResetType, error)
	// Reset issues a reset of the given type to the ComputerSystem with the
	// given Redfish ID.
	Reset(systemID string, resetType schemas.ResetType) error
}

// GenericClient is the default, vendor-agnostic implementation of Client backed
// by gofish. It performs plain Redfish operations and is the fallback whenever a
// BMC's vendor is unknown or has no registered plugin.
type GenericClient struct {
	api    *gofish.APIClient
	vendor Vendor
}

// NewGenericClient wraps a connected gofish client as a GenericClient with the
// given detected vendor.
func NewGenericClient(api *gofish.APIClient, vendor Vendor) *GenericClient {
	if vendor == "" {
		vendor = VendorGeneric
	}
	return &GenericClient{api: api, vendor: vendor}
}

func (g *GenericClient) Gofish() *gofish.APIClient { return g.api }

func (g *GenericClient) Vendor() Vendor { return g.vendor }

func (g *GenericClient) Logout() {
	if g.api != nil {
		g.api.Logout()
	}
}

// systemByID looks up a ComputerSystem under the ServiceRoot by its Redfish ID.
func (g *GenericClient) systemByID(systemID string) (*schemas.ComputerSystem, error) {
	systems, err := g.api.GetService().Systems()
	if err != nil {
		return nil, err
	}
	for i := range systems {
		if systems[i].ID == systemID {
			return systems[i], nil
		}
	}
	return nil, fmt.Errorf("computer system %q not found", systemID)
}

func (g *GenericClient) GetPowerState(systemID string) (schemas.PowerState, error) {
	system, err := g.systemByID(systemID)
	if err != nil {
		return "", err
	}
	return system.PowerState, nil
}

func (g *GenericClient) GetResetTypes(systemID string) ([]schemas.ResetType, error) {
	system, err := g.systemByID(systemID)
	if err != nil {
		return nil, err
	}
	return system.GetSupportedResetTypes()
}

func (g *GenericClient) Reset(systemID string, resetType schemas.ResetType) error {
	system, err := g.systemByID(systemID)
	if err != nil {
		return err
	}
	// gofish v0.22's Reset returns a *schemas.TaskMonitorInfo for async tracking;
	// the generic client preserves error-only semantics for now. The task handle
	// is where confirmation/polling (bugs.md power parity #4) will hook in later.
	_, err = system.Reset(resetType)
	return err
}

// ErrUnsupportedQuirk is the canonical "fail loudly" error a vendor plugin (or
// the generic fallback) returns when it encounters an operation it cannot
// safely perform without vendor-specific handling. Failing loudly signals that
// a vendor-specific plugin under pkg/bmc/vendors is required.
func ErrUnsupportedQuirk(vendor Vendor, op string) error {
	return fmt.Errorf(
		"operation %q is not supported by the %q Redfish client without vendor-specific handling; "+
			"a vendor plugin under pkg/bmc/vendors is required (please file an issue or submit a PR)",
		op, vendor,
	)
}
