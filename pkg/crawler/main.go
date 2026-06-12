package crawler

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

// CrawlerConfig is an alias for bmc.ConnConfig, the canonical BMC connection
// configuration. It is retained for backwards compatibility with existing
// callers and tests; its GetUserPass method is defined on bmc.ConnConfig.
type CrawlerConfig = bmc.ConnConfig

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
	URI                    string              `json:"uri,omitempty"`
	UUID                   string              `json:"uuid,omitempty"`
	Name                   string              `json:"name,omitempty"`
	Description            string              `json:"description,omitempty"`
	Model                  string              `json:"model,omitempty"`
	Type                   string              `json:"type,omitempty"`
	FirmwareVersion        string              `json:"firmware_version,omitempty"`
	EthernetInterfaces     []EthernetInterface `json:"ethernet_interfaces,omitempty"`
	SerialConsoleSupported []string            `json:"serial_console"`
	CommandShellSupported  []string            `json:"command_shell"`
}

type Links struct {
	Chassis  []string `json:"chassis,omitempty"`
	Managers []string `json:"managers,omitempty"`
}

type Power struct {
	State           string   `json:"state,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	RestorePolicy   string   `json:"restore_policy,omitempty"`
	PowerControlIDs []string `json:"power_control_ids,omitempty"`
}

type SerialConsoleConfig struct {
	Port    uint `json:"port,omitempty"`
	Enabled bool `json:"enabled,omitempty"`
}

type SerialConsole struct {
	IPMI   SerialConsoleConfig `json:"impi,omitempty"`
	Telnet SerialConsoleConfig `json:"telnet,omitempty"`
	SSH    SerialConsoleConfig `json:"ssh,omitempty"`
}

type InventoryDetail struct {
	URI                  string              `json:"uri,omitempty"`                  // URI of the BMC
	UUID                 string              `json:"uuid,omitempty"`                 // UUID of Node
	Manufacturer         string              `json:"manufacturer,omitempty"`         // Manufacturer of the Node
	SystemType           string              `json:"system_type,omitempty"`          // System type of the Node
	Name                 string              `json:"name,omitempty"`                 // Name of the Node
	ModelNumber          string              `json:"model,omitempty"`                // Model of the Node
	SerialNumber         string              `json:"serial,omitempty"`               // Serial number of the Node
	SerialConsole        SerialConsole       `json:"serial_console,omitempty"`       // Supported serial console types of the Node
	BiosVersion          string              `json:"bios_version,omitempty"`         // Version of the BIOS
	EthernetInterfaces   []EthernetInterface `json:"ethernet_interfaces,omitempty"`  // Ethernet interfaces of the Node
	NetworkInterfaces    []NetworkInterface  `json:"network_interfaces,omitempty"`   // Network interfaces of the Node
	Actions              []string            `json:"actions,omitempty"`              // Available actions for Node
	Power                Power               `json:"power,omitempty"`                // Power related settings of Node
	ProcessorCount       uint                `json:"processor_count,omitempty"`      // Processors of the Node
	ProcessorType        string              `json:"processor_type,omitempty"`       // Processor type of the Node
	MemoryTotal          float64             `json:"memory_total,omitempty"`         // Total memory of the Node in Gigabytes
	TrustedModules       []string            `json:"trusted_modules,omitempty"`      // Trusted modules of the Node
	TrustedComponents    []string            `json:"trusted_components,omitempty"`   // Trusted components of the Chassis
	Chassis_SKU          string              `json:"chassis_sku,omitempty"`          // SKU of the Chassis
	Chassis_Serial       string              `json:"chassis_serial,omitempty"`       // Serial number of the Chassis
	Chassis_AssetTag     string              `json:"chassis_asset_tag,omitempty"`    // Asset tag of the Chassis
	Chassis_Manufacturer string              `json:"chassis_manufacturer,omitempty"` // Manufacturer of the Chassis
	Chassis_Model        string              `json:"chassis_model,omitempty"`        // Model of the Chassis
	Links                Links               `json:"links,omitempty"`                // Links to specific resources
	NodeID               string              `json:"node_id,omitempty"`              // Node ID within the BMC, e.g. /redfish/v1/Systems/<ID>
}

// GetBMCClient connects to a BMC (Baseboard Management Controller) using the provided configuration,
// and returns the active client.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//
// Returns:
//   - *gofish.APIClient: The active client for the BMC.
//   - error: An error object if any error occurs during the connection or retrieval process.
//
// The function performs the following steps:
//  1. Initializes a gofish client with the provided configuration.
//  2. Attempts to connect to the BMC using the gofish client.
//  3. Handles specific connection errors such as 404 (ServiceRoot not found) and 401 (authentication failed).
//  4. Returns the active gofish client.
func GetBMCClient(config CrawlerConfig) (*gofish.APIClient, error) {
	// Delegate to the shared BMC manager, which is the single point where
	// gofish.Connect is called and where credential loading and error
	// decoration happen.
	return bmc.DefaultManager.Connect(config)
}

// CrawlBMCForSystems pulls all pertinent information from a BMC.
// It accepts a CrawlerConfig and returns a list of InventoryDetail structs.
func CrawlBMCForSystems(config CrawlerConfig) ([]InventoryDetail, error) {
	var (
		systems    = make(map[string]*InventoryDetail)
		rf_systems []*schemas.ComputerSystem
	)

	client, err := GetBMCClient(config)
	if err != nil {
		return []InventoryDetail{}, err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()
	log.Debug().
		Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	rf_managers, err := rf_service.Managers()
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to get managers from ServiceRoot")
	}
	return walkManagers(rf_managers, config.URI)
}

// walkSystems processes a list of Redfish computer systems and their associated chassis,
// and returns a list of inventory details for each system.
//
// Parameters:
//   - rf_systems: A slice of pointers to schemas.ComputerSystem objects representing the computer systems to be processed.
//   - rf_chassis: A pointer to a schemas.Chassis object representing the chassis associated with the computer systems.
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
func walkSystems(rf_systems []*schemas.ComputerSystem, rf_chassis *schemas.Chassis, baseURI string) ([]InventoryDetail, error) {
	systems := []InventoryDetail{}
	for _, rf_computersystem := range rf_systems {
		var (
			managerLinks    []string
			chassisLinks    []string
			power           *schemas.Power
			powercontrolIDs []string
		)

		// get all of the links to managers
		rf_managers, err := rf_computersystem.ManagedBy()
		if err != nil {
			log.Warn().
				Err(err).
				Msg("failed to get system managers")
			log.Error().
				Err(err).
				Str("id", rf_computersystem.ID).
				Str("system", rf_computersystem.Name).
				Msg("failed to get manager for system")
		} else {
			for _, manager := range rf_managers {
				managerLinks = append(managerLinks, manager.ODataID)
			}
		}

		if rf_chassis != nil {
			chassisLinks = append(chassisLinks, rf_chassis.ODataID)

			// get power-related details from rf_chassis
			power, err = rf_chassis.Power()
			if err != nil {
				log.Warn().Err(err).Str("id", rf_computersystem.ID).
					Str("system", rf_computersystem.Name).Msg("failed to get power-related details from chassis")
			} else {
				// extract the power control odata.id resource
				if power != nil {
					for _, rf_powercontrol := range power.PowerControl {
						powercontrolIDs = append(powercontrolIDs, rf_powercontrol.ODataID)
					}
				}
			}
		}

		// convert supported reset types to []string
		var (
			resetTypes []schemas.ResetType
			actions    []string
		)
		resetTypes, err = rf_computersystem.GetSupportedResetTypes()
		if err != nil {
			log.Warn().Err(err).Str("system", rf_computersystem.Name).Msg("failed to get supported reset types for system")
		}
		for _, action := range resetTypes {
			actions = append(actions, string(action))
		}

		// get all of the links to the chassis
		system := InventoryDetail{
			URI:          baseURI + "/redfish/v1/Systems/" + rf_computersystem.ID,
			UUID:         rf_computersystem.UUID,
			Name:         rf_computersystem.Name,
			Manufacturer: rf_computersystem.Manufacturer,
			SystemType:   string(rf_computersystem.SystemType),
			ModelNumber:  rf_computersystem.Model,
			SerialNumber: rf_computersystem.SerialNumber,
			SerialConsole: SerialConsole{
				IPMI: SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.IPMI.ServiceEnabled,
				},
				SSH: SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.SSH.ServiceEnabled,
				},
				Telnet: SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.Telnet.ServiceEnabled,
				},
			},
			BiosVersion: rf_computersystem.BiosVersion,
			Links: Links{
				Managers: managerLinks,
				Chassis:  chassisLinks,
			},
			Power: Power{
				Mode:            string(rf_computersystem.PowerMode),
				State:           string(rf_computersystem.PowerState),
				RestorePolicy:   string(rf_computersystem.PowerRestorePolicy),
				PowerControlIDs: powercontrolIDs,
			},
			Actions:       actions,
			ProcessorType: rf_computersystem.ProcessorSummary.Model,
			NodeID:        rf_computersystem.ID,
		}

		// check that pointers values are set before de-referencing
		if rf_computersystem.SerialConsole.IPMI.Port != nil {
			system.SerialConsole.IPMI.Port = uint(*rf_computersystem.SerialConsole.IPMI.Port)
		}
		if rf_computersystem.SerialConsole.SSH.Port != nil {
			system.SerialConsole.SSH.Port = uint(*rf_computersystem.SerialConsole.SSH.Port)
		}
		if rf_computersystem.SerialConsole.Telnet.Port != nil {
			system.SerialConsole.Telnet.Port = uint(*rf_computersystem.SerialConsole.Telnet.Port)
		}
		if rf_computersystem.ProcessorSummary.Count != nil {
			system.ProcessorCount = uint(*rf_computersystem.ProcessorSummary.Count)
		}
		if rf_computersystem.MemorySummary.TotalSystemMemoryGiB != nil {
			system.MemoryTotal = float64(*rf_computersystem.MemorySummary.TotalSystemMemoryGiB)
		}

		if rf_chassis != nil {
			system.Chassis_SKU = rf_chassis.SKU
			system.Chassis_Serial = rf_chassis.SerialNumber
			system.Chassis_AssetTag = rf_chassis.AssetTag
			system.Chassis_Manufacturer = rf_chassis.Manufacturer
			system.Chassis_Model = rf_chassis.Model
		}

		// add ethernet interfaces
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

		// add network interfaces
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

		// TrustedModules is retained for compatibility with older Redfish services.
		//nolint:staticcheck
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
//	rf_managers - A slice of pointers to schemas.Manager objects representing the Redfish managers to be processed.
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
func walkManagers(rf_managers []*schemas.Manager, baseURI string) ([]Manager, error) {
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

		var supported_serial_console []string
		// Manager.SerialConsole is retained for compatibility with older services.
		//nolint:staticcheck
		for _, console_type := range rf_manager.SerialConsole.ConnectTypesSupported {
			supported_serial_console = append(supported_serial_console, string(console_type))
		}
		var supported_command_shell []string
		for _, shell_type := range rf_manager.CommandShell.ConnectTypesSupported {
			supported_command_shell = append(supported_command_shell, string(shell_type))
		}

		managers = append(managers, Manager{
			URI:                    baseURI + "/redfish/v1/Managers/" + rf_manager.ID,
			UUID:                   rf_manager.UUID,
			Name:                   rf_manager.Name,
			Description:            rf_manager.Description,
			Model:                  rf_manager.Model,
			Type:                   string(rf_manager.ManagerType),
			FirmwareVersion:        rf_manager.FirmwareVersion,
			EthernetInterfaces:     ethernet_interfaces,
			SerialConsoleSupported: supported_serial_console,
			CommandShellSupported:  supported_command_shell,
		})
	}
	return managers, nil
}
func extractPtrMapValues[T any](m map[string]*T) []T {
	slice := make([]T, 0, len(m))
	for i := range m {
		slice = append(slice, *m[i])
	}
	return slice
}

func merge(systems map[string]*InventoryDetail, newSystems []InventoryDetail) map[string]*InventoryDetail {
	// add and replace values in systems with values from newSystems
	for _, system := range newSystems {
		systems[system.URI] = &system
	}
	return systems
}
