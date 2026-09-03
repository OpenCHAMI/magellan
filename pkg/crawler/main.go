package crawler

import (
	"fmt"
	"strings"

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
	Port    int  `json:"port,omitempty"`
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
	log.Debug().Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	// Nodes are sometimes only found under Chassis, but they should be found under Systems.
	rf_chassis, err := rf_service.Chassis()
	if err == nil {
		log.Debug().Msgf("found %d chassis in ServiceRoot", len(rf_chassis))
		for _, chassis := range rf_chassis {
			rf_chassis_systems, err := chassis.ComputerSystems()
			if err == nil {
				// rf_systems = append(rf_systems, rf_chassis_systems...)
				log.Debug().Msgf("found %d systems in chassis %s", len(rf_chassis_systems), chassis.ID)
			}

			// Walk the systems found under Chassis with reference
			newSystems, err := walkSystems(rf_chassis_systems, chassis, config.URI)
			if err != nil {
				log.Error().
					Err(err).
					Str("chassis_id", chassis.ID).
					Str("uri", config.URI).
					Msg("failed to get systems in chassis...continuing...")
				continue
			}

			// add systems found from chassis to total collection
			for i := range newSystems {
				systems[newSystems[i].URI] = &newSystems[i]
			}
		}
	}
	rf_root_systems, err := rf_service.Systems()
	if err != nil {
		log.Error().Err(err).Msg("failed to get systems from ServiceRoot")
	}
	log.Debug().Msgf("found %d systems in ServiceRoot", len(rf_root_systems))
	rf_systems = append(rf_systems, rf_root_systems...)
	newSystems, err := walkSystems(rf_systems, nil, config.URI)
	if err != nil {
		return extractPtrMapValues(systems), fmt.Errorf("failed to get systems: %v", err)
	}
	// If nodes are found under both Chassis and Systems, Systems is assumed to be "more definitive"
	// and will override corresponding fields from the Chassis version.
	systems = merge(systems, newSystems)
	return extractPtrMapValues(systems), nil
}

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
//  1. Creates a logged-in gofish client for the BMC with the provided configuration.
//  2. Logs out from the client after the operations are completed.
//  3. Retrieves the ServiceRoot from the connected BMC.
//  4. Fetches the list of managers from the ServiceRoot.
//  5. Returns the list of managers and any error encountered during the process.
func CrawlBMCForManagers(config CrawlerConfig) ([]Manager, error) {
	var managers []Manager
	client, err := GetBMCClient(config)
	if err != nil {
		return managers, err
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
		actions := []string{}
		supportedResetTypes, _ := rf_computersystem.GetSupportedResetTypes()
		for _, action := range supportedResetTypes {
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
					Port:    derefUint(rf_computersystem.SerialConsole.IPMI.Port),
					Enabled: rf_computersystem.SerialConsole.IPMI.ServiceEnabled,
				},
				SSH: SerialConsoleConfig{
					Port:    derefUint(rf_computersystem.SerialConsole.SSH.Port),
					Enabled: rf_computersystem.SerialConsole.SSH.ServiceEnabled,
				},
				Telnet: SerialConsoleConfig{
					Port:    derefUint(rf_computersystem.SerialConsole.Telnet.Port),
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
			Actions:        actions,
			ProcessorCount: derefUint(rf_computersystem.ProcessorSummary.Count),
			ProcessorType:  rf_computersystem.ProcessorSummary.Model,
			MemoryTotal:    derefFloat(rf_computersystem.MemorySummary.TotalSystemMemoryGiB),
			NodeID:         rf_computersystem.ID,
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
				IP:          firstIPAddress(rf_ethernetinterface),
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

		//nolint:staticcheck // Preserve legacy TrustedModules inventory data for existing consumers.
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
			ip := firstIPAddress(rf_ethernetinterface)
			if ip == "" {
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
		//nolint:staticcheck // Manager serial-console data remains part of Magellan's manager inventory contract.
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

// func getPowerInfo(serviceroot *gofish.Service) ([]Power, error) {
// 	// get the power control related information (Actions, URL, PowerControl, Links, etc.)

// 	// get the SupportedResetTypes from /redfish/v1/Systems
// 	// get the Power/PowerControl from /redfish/v1/Chassis
// 	rf_chassis, err := serviceroot.Chassis()
// 	if err != nil {

// 	}

// 	power := []Power{}
// 	for _, chassis := range rf_chassis {
// 		rf_power, err := chassis.Power()
// 		if err != nil {

// 		}
// 		rf_computersystems, err := chassis.ComputerSystems()
// 		if err != nil {

// 		}

// 		for _, computersystem := range rf_computersystems {
// 			computersystem.SupportedResetTypes
// 		}

// 		power = append(power, Power{
// 			URL: "",
// 			Control: PowerControl{
// 				MemberID:     "",
// 				ResetTypes:   rf_computersystem.SupportedResetTypes,
// 				RelatedItems: []string{},
// 			},
// 		})
// 	}

// }

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

// firstIPAddress returns the interface's first non-empty address, preferring
// IPv4. BMCs commonly present an IPv6-only management NIC, and some pad the
// address arrays with blank entries, so neither array length nor index 0 is a
// reliable indicator on its own.
func firstIPAddress(iface *schemas.EthernetInterface) string {
	if iface == nil {
		return ""
	}
	for _, addr := range iface.IPv4Addresses {
		if v := strings.TrimSpace(addr.Address); v != "" {
			return v
		}
	}
	for _, addr := range iface.IPv6Addresses {
		if v := strings.TrimSpace(addr.Address); v != "" {
			return v
		}
	}
	return ""
}

// derefUint dereferences an optional *uint Redfish field to an int, yielding 0
// when the BMC omitted the value. gofish v0.22 pointer-ized these optional
// numeric fields; treating nil as 0 preserves the pre-upgrade output.
func derefUint(p *uint) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

// derefFloat dereferences an optional *float64 Redfish field to a float32,
// yielding 0 when the BMC omitted the value (see derefUint).
func derefFloat(p *float64) float32 {
	if p == nil {
		return 0
	}
	return float32(*p)
}
