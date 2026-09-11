// Package vendors blank-imports all in-tree BMC vendor plugins so that their
// init() registrations run. Import this package for its side effects:
//
//	import _ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors"
//
// Vendors with no matching plugin fall back to the generic Redfish client.
package vendors

import (
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors/cray"
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors/dell"
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors/hpe"
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors/lenovo"
	_ "github.com/OpenCHAMI/magellan/pkg/bmc/vendors/supermicro"
)
