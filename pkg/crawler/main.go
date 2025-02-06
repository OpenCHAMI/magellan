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
	Enabled     bool   `json:"enabled,omitempty"`     // Enabled interface
}

type NetworkAdapter struct {
	URI          string `json:"uri,omitempty"`          // URI of the adapter
	Manufacturer string `json:"manufacturer,omitempty"` // Manufacturer of the adapter
	Name         string `json:"name,omitempty"`         // Name of the adapter
	Model        string `json:"model,omitempty"`        // Model of the adapter
	Serial       string `json:"serial,omitempty"`       // Serial number of the adapter
	Description  string `json:"description,omitempty"`  // Description of the adapter
}

type NetworkInterface struct {
	URI         string         `json:"uri,omitempty"`         // URI of the interface
	Name        string         `json:"name,omitempty"`        // Name of the interface
	Description string         `json:"description,omitempty"` // Description of the interface
	Adapter     NetworkAdapter `json:"adapter,omitempty"`     // Adapter of the interface
}

type Manager struct {
	URI                string              `json:"uri,omitempty"`
	UUID               string              `json:"uuid,omitempty"`
	Name               string              `json:"name,omitempty"`
	Description        string              `json:"description,omitempty"`
	Model              string              `json:"model,omitempty"`
	Type               string              `json:"type,omitempty"`
	FirmwareVersion    string              `json:"firmware_version,omitempty"`
	EthernetInterfaces []EthernetInterface `json:"ethernet_interfaces,omitempty"`
}

type InventoryDetail struct {
	URI                  string              `json:"uri,omitempty"`                  // URI of the BMC
	UUID                 string              `json:"uuid,omitempty"`                 // UUID of Node
	Manufacturer         string              `json:"manufacturer,omitempty"`         // Manufacturer of the Node
	SystemType           string              `json:"system_type,omitempty"`          // System type of the Node
	Name                 string              `json:"name,omitempty"`                 // Name of the Node
	Model                string              `json:"model,omitempty"`                // Model of the Node
	Serial               string              `json:"serial,omitempty"`               // Serial number of the Node
	BiosVersion          string              `json:"bios_version,omitempty"`         // Version of the BIOS
	EthernetInterfaces   []EthernetInterface `json:"ethernet_interfaces,omitempty"`  // Ethernet interfaces of the Node
	NetworkInterfaces    []NetworkInterface  `json:"network_interfaces,omitempty"`   // Network interfaces of the Node
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

// CrawlBMCForSystems pulls all pertinent information from a BMC.  It accepts a CrawlerConfig and returns a list of InventoryDetail structs.
func CrawlBMCForSystems(config CrawlerConfig) ([]InventoryDetail, error) {
	var (
		systems    []InventoryDetail
		rf_systems []*redfish.ComputerSystem
	)
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
	return walkSystems(rf_systems, nil, config.URI)
}

// CrawlBMCForSystems pulls BMC manager information.
// CrawlBMCForManagers connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and then fetches the list of managers from the ServiceRoot.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//
// Returns:
//   - []Manager: A slice of Manager structs representing the managers retrieved from the BMC.
//   - error: An error object if any error occurs during the connection or retrieval process.
//
// The function performs the following steps:
//  1. Initializes a gofish client with the provided configuration.
//  2. Attempts to connect to the BMC using the gofish client.
//  3. Handles specific connection errors such as 404 (ServiceRoot not found) and 401 (authentication failed).
//  4. Logs out from the client after the operations are completed.
//  5. Retrieves the ServiceRoot from the connected BMC.
//  6. Fetches the list of managers from the ServiceRoot.
//  7. Returns the list of managers and any error encountered during the process.
func CrawlBMCForManagers(config CrawlerConfig) ([]Manager, error) {
	// initialize gofish client
	var managers []Manager
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
		return managers, err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()
	log.Info().Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	rf_managers, err := rf_service.Managers()
	if err != nil {
		log.Error().Err(err).Msg("failed to get managers from ServiceRoot")
	}
	return walkManagers(rf_managers, config.URI)
}

// walkSystems processes a list of Redfish computer systems and their associated chassis,
// and returns a list of inventory details for each system.
//
// Parameters:
//   - rf_systems: A slice of pointers to redfish.ComputerSystem objects representing the computer systems to be processed.
//   - rf_chassis: A pointer to a redfish.Chassis object representing the chassis associated with the computer systems.
//   - baseURI: A string representing the base URI for constructing resource URIs.
//
// Returns:
//   - A slice of InventoryDetail objects containing detailed information about each computer system.
//   - An error if any issues occur while processing the computer systems or their associated resources.
//
// The function performs the following steps:
//  1. Iterates over each computer system in rf_systems.
//  2. Constructs an InventoryDetail object for each computer system, populating fields such as URI, UUID, Name, Manufacturer, SystemType, Model, Serial, BiosVersion, PowerState, ProcessorCount, ProcessorType, and MemoryTotal.
//  3. If rf_chassis is not nil, populates additional chassis-related fields in the InventoryDetail object.
//  4. Retrieves and processes Ethernet interfaces for each computer system, adding them to the EthernetInterfaces field of the InventoryDetail object.
//  5. Retrieves and processes Network interfaces and their associated network adapters for each computer system, adding them to the NetworkInterfaces field of the InventoryDetail object.
//  6. Processes trusted modules for each computer system, adding them to the TrustedModules field of the InventoryDetail object.
//  7. Appends the populated InventoryDetail object to the systems slice.
//  8. Returns the systems slice and any error encountered during processing.
func walkSystems(rf_systems []*redfish.ComputerSystem, rf_chassis *redfish.Chassis, baseURI string) ([]InventoryDetail, error) {
	systems := []InventoryDetail{}
	for _, rf_computersystem := range rf_systems {
		system := InventoryDetail{
			URI:            baseURI + "/redfish/v1/Systems/" + rf_computersystem.ID,
			UUID:           rf_computersystem.UUID,
			Name:           rf_computersystem.Name,
			Manufacturer:   rf_computersystem.Manufacturer,
			SystemType:     string(rf_computersystem.SystemType),
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
				URI:         baseURI + rf_ethernetinterface.ODataID,
				MAC:         rf_ethernetinterface.MACAddress,
				Name:        rf_ethernetinterface.Name,
				Description: rf_ethernetinterface.Description,
				Enabled:     rf_ethernetinterface.InterfaceEnabled,
			}
			if len(rf_ethernetinterface.IPv4Addresses) > 0 {
				ethernetinterface.IP = rf_ethernetinterface.IPv4Addresses[0].Address
			}
			system.EthernetInterfaces = append(system.EthernetInterfaces, ethernetinterface)
		}

		rf_networkInterfaces, err := rf_computersystem.NetworkInterfaces()
		if err != nil {
			log.Error().Err(err).Msg("failed to get network interfaces from computer system")
			return systems, err
		}

		for _, rf_networkInterface := range rf_networkInterfaces {
			rf_networkAdapter, err := rf_networkInterface.NetworkAdapter()
			if err != nil {
				log.Error().Err(err).Msg("failed to get network adapter from network interface")
				return systems, err
			}

			var networkAdapter NetworkAdapter
			if rf_networkAdapter != nil {
				networkAdapter = NetworkAdapter{
					URI:          baseURI + rf_networkAdapter.ODataID,
					Name:         rf_networkAdapter.Name,
					Manufacturer: rf_networkAdapter.Manufacturer,
					Model:        rf_networkAdapter.Model,
					Serial:       rf_networkAdapter.SerialNumber,
					Description:  rf_networkAdapter.Description,
				}
			}

			networkInterface := NetworkInterface{
				URI:         baseURI + rf_networkInterface.ODataID,
				Name:        rf_networkInterface.Name,
				Description: rf_networkInterface.Description,
				Adapter:     networkAdapter,
			}
			system.NetworkInterfaces = append(system.NetworkInterfaces, networkInterface)
		}

		for _, rf_trustedmodule := range rf_computersystem.TrustedModules {
			system.TrustedModules = append(system.TrustedModules, fmt.Sprintf("%s %s", rf_trustedmodule.InterfaceType, rf_trustedmodule.FirmwareVersion))
		}

		systems = append(systems, system)
	}
	return systems, nil
}

// walkManagers processes a list of Redfish managers and extracts relevant information
// to create a slice of Manager objects.
//
// Parameters:
//
//	rf_managers - A slice of pointers to redfish.Manager objects representing the Redfish managers to be processed.
//	baseURI - A string representing the base URI to be used for constructing URIs for the managers and their Ethernet interfaces.
//
// Returns:
//
//	A slice of Manager objects containing the extracted information from the provided Redfish managers.
//	An error if any issues occur while retrieving Ethernet interfaces from the managers.
//
// The function iterates over each Redfish manager, retrieves its Ethernet interfaces,
// and constructs a Manager object with the relevant details, including Ethernet interface information.
// If an error occurs while retrieving Ethernet interfaces, the function logs the error and returns the managers
// collected so far along with the error.
func walkManagers(rf_managers []*redfish.Manager, baseURI string) ([]Manager, error) {
	var managers []Manager
	for _, rf_manager := range rf_managers {
		rf_ethernetinterfaces, err := rf_manager.EthernetInterfaces()
		if err != nil {
			log.Error().Err(err).Msg("failed to get ethernet interfaces from manager")
			return managers, err
		}
		var ethernet_interfaces []EthernetInterface
		for _, rf_ethernetinterface := range rf_ethernetinterfaces {
			if len(rf_ethernetinterface.IPv4Addresses) <= 0 {
				continue
			}
			ethernet_interfaces = append(ethernet_interfaces, EthernetInterface{
				URI:         baseURI + rf_ethernetinterface.ODataID,
				MAC:         rf_ethernetinterface.MACAddress,
				Name:        rf_ethernetinterface.Name,
				Description: rf_ethernetinterface.Description,
				Enabled:     rf_ethernetinterface.InterfaceEnabled,
				IP:          rf_ethernetinterface.IPv4Addresses[0].Address,
			})
		}
		managers = append(managers, Manager{
			URI:                baseURI + "/redfish/v1/Managers/" + rf_manager.ID,
			UUID:               rf_manager.UUID,
			Name:               rf_manager.Name,
			Description:        rf_manager.Description,
			Model:              rf_manager.Model,
			Type:               string(rf_manager.ManagerType),
			FirmwareVersion:    rf_manager.FirmwareVersion,
			EthernetInterfaces: ethernet_interfaces,
		})
	}
	return managers, nil
}
