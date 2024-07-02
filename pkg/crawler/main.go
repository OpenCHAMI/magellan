package crawler

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

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
	URI                  string              `json:"uri,omitempty"`                  // URI of the BMC
	Manufacturer         string              `json:"manufacturer,omitempty"`         // Manufacturer of the Node
	Name                 string              `json:"name,omitempty"`                 // Name of the Node
	Model                string              `json:"model,omitempty"`                // Model of the Node
	Serial               string              `json:"serial,omitempty"`               // Serial number of the Node
	BiosVersion          string              `json:"bios_version,omitempty"`         // Version of the BIOS
	EthernetInterfaces   []EthernetInterface `json:"ethernet_interfaces,omitempty"`  // Ethernet interfaces of the Node
	PowerState           string              `json:"power_state,omitempty"`          // Power state of the Node
	ProcessorCount       int                 `json:"processor_count,omitempty"`      // Processors of the Node
	ProcessorType        string              `json:"processor_type,omitempty"`       // Processor type of the Node
	MemoryTotal          float32             `json:"memory_total,omitempty"`         // Total memory of the Node in Gigabytes
	TrustedModules       []string            `json:"trusted_modules,omitempty"`      // Trusted modules of the Node
	TrustedComponents    []string            `json:"trusted_components,omitempty"`   // Trusted components of the Chassis
	Chassis_SKU          string              `json:"chassis_sku,omitempty"`          // SKU of the Chassis
	Chassis_Serial       string              `json:"chassis_serial,omitempty"`       // Serial number of the Chassis
	Chassis_AssetTag     string              `json:"chassis_asset_tag,omitempty"`    // Asset tag of the Chassis
	Chassis_Manufacturer string              `json:"chassis_manufacturer,omitempty"` // Manufacturer of the Chassis
	Chassis_Model        string              `json:"chassis_model,omitempty"`        // Model of the Chassis
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
		if strings.HasPrefix(err.Error(), "404:") {
			err = fmt.Errorf("no ServiceRoot found.  This is probably not a BMC: %s", config.URI)
		}
		if strings.HasPrefix(err.Error(), "401:") {
			err = fmt.Errorf("authentication failed.  Check your username and password: %s", config.URI)
		}
		event := log.Error()
		event.Err(err)
		event.Msg("failed to connect to BMC")
		return systems, err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()
	log.Info().Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	var rf_systems []*redfish.ComputerSystem

	// Nodes are sometimes only found under Chassis, but they should be found under Systems.
	rf_chassis, err := rf_service.Chassis()
	if err == nil {
		log.Info().Msgf("found %d chassis in ServiceRoot", len(rf_chassis))
		for _, chassis := range rf_chassis {
			rf_chassis_systems, err := chassis.ComputerSystems()
			if err == nil {
				rf_systems = append(rf_systems, rf_chassis_systems...)
				log.Info().Msgf("found %d systems in chassis %s", len(rf_chassis_systems), chassis.ID)
			}
		}
	}
	rf_root_systems, err := rf_service.Systems()
	if err != nil {
		log.Error().Err(err).Msg("failed to get systems from ServiceRoot")
	}
	log.Info().Msgf("found %d systems in ServiceRoot", len(rf_root_systems))
	rf_systems = append(rf_systems, rf_root_systems...)
	systems, err = walkSystems(rf_systems, nil, config.URI)
	return systems, err
}

func walkSystems(rf_systems []*redfish.ComputerSystem, rf_chassis *redfish.Chassis, baseURI string) ([]InventoryDetail, error) {
	systems := []InventoryDetail{}
	for _, rf_computersystem := range rf_systems {
		system := InventoryDetail{
			URI:            baseURI + "/redfish/v1/Systems/" + rf_computersystem.ID,
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
		if rf_chassis != nil {
			system.Chassis_SKU = rf_chassis.SKU
			system.Chassis_Serial = rf_chassis.SerialNumber
			system.Chassis_AssetTag = rf_chassis.AssetTag
			system.Chassis_Manufacturer = rf_chassis.Manufacturer
			system.Chassis_Model = rf_chassis.Model
		}

		rf_ethernetinterfaces, err := rf_computersystem.EthernetInterfaces()
		if err != nil {
			log.Error().Err(err).Msg("failed to get ethernet interfaces from computer system")
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
		for _, rf_trustedmodule := range rf_computersystem.TrustedModules {
			system.TrustedModules = append(system.TrustedModules, fmt.Sprintf("%s %s", rf_trustedmodule.InterfaceType, rf_trustedmodule.FirmwareVersion))
		}
		systems = append(systems, system)
	}
	return systems, nil
}
