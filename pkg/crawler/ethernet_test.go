package crawler

import (
	"encoding/json"
	"testing"

	"github.com/stmcginnis/gofish/schemas"
	"github.com/stretchr/testify/require"
)

func TestMapEthernetInterfaces(t *testing.T) {
	const baseURI = "https://bmc.example"

	newRFInterface := func(odataID string, addrs ...string) *schemas.EthernetInterface {
		rf_ethernetinterface := &schemas.EthernetInterface{}
		rf_ethernetinterface.ODataID = odataID
		rf_ethernetinterface.Name = "eth0"
		rf_ethernetinterface.Description = "test nic"
		rf_ethernetinterface.InterfaceEnabled = true
		rf_ethernetinterface.MACAddress = "aa:bb:cc:dd:ee:ff"
		for _, addr := range addrs {
			rf_ethernetinterface.IPv4Addresses = append(rf_ethernetinterface.IPv4Addresses,
				schemas.IPv4Address{Address: addr})
		}
		return rf_ethernetinterface
	}

	t.Run("keeps interface with usable IP", func(t *testing.T) {
		got := mapEthernetInterfaces([]*schemas.EthernetInterface{
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/0", "10.0.0.5"),
		}, baseURI)

		require.Len(t, got, 1)
		require.Equal(t, EthernetInterface{
			URI:         baseURI + "/redfish/v1/Systems/1/EthernetInterfaces/0",
			MAC:         "aa:bb:cc:dd:ee:ff",
			IP:          "10.0.0.5",
			Name:        "eth0",
			Description: "test nic",
			Enabled:     true,
		}, got[0])

		// the ip field must survive JSON serialization: it is the field
		// SMD validates (OpenCHAMI/magellan#191).
		out, err := json.Marshal(got[0])
		require.NoError(t, err)
		require.Contains(t, string(out), `"ip":"10.0.0.5"`)
	})

	t.Run("skips interface without addresses", func(t *testing.T) {
		got := mapEthernetInterfaces([]*schemas.EthernetInterface{
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/0"),
		}, baseURI)
		require.Empty(t, got)
	})

	t.Run("skips empty address", func(t *testing.T) {
		got := mapEthernetInterfaces([]*schemas.EthernetInterface{
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/0", ""),
		}, baseURI)
		require.Empty(t, got)
	})

	t.Run("skips unparsable address", func(t *testing.T) {
		got := mapEthernetInterfaces([]*schemas.EthernetInterface{
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/0", "Not Available"),
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/1", "x0c0s0b0"),
		}, baseURI)
		require.Empty(t, got)
	})

	t.Run("uses first parsable address", func(t *testing.T) {
		got := mapEthernetInterfaces([]*schemas.EthernetInterface{
			newRFInterface("/redfish/v1/Systems/1/EthernetInterfaces/0", "Not Available", "192.168.0.9"),
		}, baseURI)

		require.Len(t, got, 1)
		require.Equal(t, "192.168.0.9", got[0].IP)
	})

	t.Run("empty input yields empty output", func(t *testing.T) {
		require.Empty(t, mapEthernetInterfaces(nil, baseURI))
		require.Empty(t, mapEthernetInterfaces([]*schemas.EthernetInterface{}, baseURI))
	})
}
