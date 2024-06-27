package crawler

import "github.com/stmcginnis/gofish"

type CrawlerConfig struct {
	URI      string // URI of the BMC
	Username string // Username for the BMC
	Password string // Password for the BMC
	Insecure bool   // Whether to ignore SSL errors
}

type EthernetInterface struct {
	URI         string `json:"uri,omitempty"`         // URI of the interface
	MAC         string `json:"mac,omitempty"`         // MAC address of the interface
	IP          string `json:"ip,omitempty"`          // IP address of the interface
	Name        string `json:"name,omitempty"`        // Name of the interface
	Description string `json:"description,omitempty"` // Description of the interface
}

type InventoryDetail struct {
	URI                string              `json:"uri,omitempty"`                 // URI of the BMC
	Manufacturer       string              `json:"manufacturer,omitempty"`        // Manufacturer of the Node
	Name               string              `json:"name,omitempty"`                // Name of the Node
	Model              string              `json:"model,omitempty"`               // Model of the Node
	Serial             string              `json:"serial,omitempty"`              // Serial number of the Node
	BiosVersion        string              `json:"bios_version,omitempty"`        // Version of the BIOS
	EthernetInterfaces []EthernetInterface `json:"ethernet_interfaces,omitempty"` // Ethernet interfaces of the Node
	PowerState         string              `json:"power_state,omitempty"`         // Power state of the Node
	ProcessorCount     int                 `json:"processor_count,omitempty"`     // Processors of the Node
	ProcessorType      string              `json:"processor_type,omitempty"`      // Processor type of the Node
	MemoryTotal        float32             `json:"memory_total,omitempty"`        // Total memory of the Node in Gigabytes
}

// CrawlBMC pulls all pertinent information from a BMC.  It accepts a CrawlerConfig and returns a list of InventoryDetail structs.
func CrawlBMC(config CrawlerConfig) ([]InventoryDetail, error) {
	var systems []InventoryDetail
	// initialize gofish client
	client, err := gofish.Connect(gofish.ClientConfig{
		Endpoint:  config.URI,
		Username:  config.Username,
		Password:  config.Password,
		Insecure:  config.Insecure,
		BasicAuth: true,
	})
	if err != nil {
		return systems, err
	}
	defer client.Logout()

	// Get the list of systems from the BMC
	rf_service := client.GetService()
	rf_systems, err := rf_service.Systems()
	if err != nil {
		return systems, err
	}
	for _, rf_computersystem := range rf_systems {
		system := InventoryDetail{
			URI:            config.URI + "/redfish/v1/Systems/" + rf_computersystem.ID,
			Name:           rf_computersystem.Name,
			Manufacturer:   rf_computersystem.Manufacturer,
			Model:          rf_computersystem.Model,
			Serial:         rf_computersystem.SerialNumber,
			BiosVersion:    rf_computersystem.BIOSVersion,
			PowerState:     string(rf_computersystem.PowerState),
			ProcessorCount: rf_computersystem.ProcessorSummary.Count,
			ProcessorType:  rf_computersystem.ProcessorSummary.Model,
			MemoryTotal:    rf_computersystem.MemorySummary.TotalSystemMemoryGiB,
		}
		// Get the list of ethernet interfaces for the system
		rf_ethernetinterfaces, err := rf_computersystem.EthernetInterfaces()
		if err != nil {
			return systems, err
		}
		for _, rf_ethernetinterface := range rf_ethernetinterfaces {
			ethernetinterface := EthernetInterface{
				MAC:         rf_ethernetinterface.MACAddress,
				Name:        rf_ethernetinterface.Name,
				Description: rf_ethernetinterface.Description,
			}
			if len(rf_ethernetinterface.IPv4Addresses) > 0 {
				ethernetinterface.IP = rf_ethernetinterface.IPv4Addresses[0].Address
			}
			system.EthernetInterfaces = append(system.EthernetInterfaces, ethernetinterface)
		}
		systems = append(systems, system)
	}
	return systems, nil
}
