package crawler

import (
	"net"
	"strings"

	"github.com/openchami/magellan/pkg/models"
	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish/schemas"
)

// mapEthernetInterfaces converts a list of Redfish ethernet interfaces into
// their magellan payload form, omitting every interface that does not report
// a usable IPv4 address.
//
// Interfaces without a usable IP are omitted rather than emitted with an
// empty ip field because consumers reject them: hms-smd validates each
// ethernet interface of a RedfishEndpoint and answers 400
// "Invalid CompEthInterface IP Address" for an empty address, failing the
// whole request (OpenCHAMI/smd#132). BMCs commonly report no IPv4Addresses
// on System interfaces at all — both emulators exercised by this project
// (sushy-tools and csm-rie) and real hardware do — which made every
// scan → collect → send pipeline fail with the strict `send` exit status
// introduced in #190 (OpenCHAMI/magellan#191).
//
// Both walkSystems and walkManagers route through this function so the
// Systems and Managers paths of the payload follow a single rule.
func mapEthernetInterfaces(rf_ethernetinterfaces []*schemas.EthernetInterface, baseURI string) []models.EthernetInterface {
	ethernet_interfaces := make([]models.EthernetInterface, 0, len(rf_ethernetinterfaces))
	for _, rf_ethernetinterface := range rf_ethernetinterfaces {
		// Dell GB200 HGX BMCs expose a usb0 interface as an internal
		// management link. It is not routable and causes SMD to reject the
		// entire endpoint with "Invalid CompEthInterface IP Address".
		if strings.HasPrefix(strings.ToLower(rf_ethernetinterface.ID), "usb") {
			log.Debug().Str("id", rf_ethernetinterface.ID).Msg("skipping USB ethernet interface")
			continue
		}
		ip := usableIPv4(rf_ethernetinterface)
		if ip == "" {
			continue
		}
		ethernet_interfaces = append(ethernet_interfaces, models.EthernetInterface{
			URI:         baseURI + rf_ethernetinterface.ODataID,
			MAC:         rf_ethernetinterface.MACAddress,
			IP:          ip,
			Name:        rf_ethernetinterface.Name,
			Description: rf_ethernetinterface.Description,
			Enabled:     rf_ethernetinterface.InterfaceEnabled,
		})
	}
	return ethernet_interfaces
}

// usableIPv4 returns the first address in rf_ethernetinterface.IPv4Addresses
// that parses as an IP address, or an empty string when the interface reports
// no addresses or none that parse (some BMCs report placeholders such as
// "Not Available").
func usableIPv4(rf_ethernetinterface *schemas.EthernetInterface) string {
	for _, address := range rf_ethernetinterface.IPv4Addresses {
		if net.ParseIP(address.Address) != nil {
			return address.Address
		}
	}
	return ""
}
