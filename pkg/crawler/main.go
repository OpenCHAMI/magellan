package crawler

import (
	"fmt"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/models"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish/schemas"
)

type CrawlerConfig struct {
	URI             string // URI of the BMC
	Insecure        bool   // Whether to ignore SSL errors
	CACertPath      string // Optional path to a trusted CA certificate
	CredentialStore secrets.SecretStore
	UseDefault      bool
}

// CrawlBMCForSystems pulls all pertinent information from a BMC.
// It accepts a CrawlerConfig and returns a list of InventoryDetail structs.
func CrawlBMCForSystems(config CrawlerConfig) ([]models.InventoryDetail, error) {
	var (
		systems    = make(map[string]*models.InventoryDetail)
		rf_systems []*schemas.ComputerSystem
	)

	client, err := bmc.ConnectWithCredentials(config.URI, config.CredentialStore, config.Insecure, config.CACertPath)
	if err != nil {
		return []models.InventoryDetail{}, err
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
func CrawlBMCForManagers(config CrawlerConfig) ([]models.Manager, error) {
	var managers []models.Manager
	client, err := bmc.ConnectWithCredentials(config.URI, config.CredentialStore, config.Insecure, config.CACertPath)
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
func walkSystems(rf_systems []*schemas.ComputerSystem, rf_chassis *schemas.Chassis, baseURI string) ([]models.InventoryDetail, error) {
	systems := []models.InventoryDetail{}
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
		system := models.InventoryDetail{
			URI:          baseURI + "/redfish/v1/Systems/" + rf_computersystem.ID,
			UUID:         rf_computersystem.UUID,
			Name:         rf_computersystem.Name,
			Manufacturer: rf_computersystem.Manufacturer,
			SystemType:   string(rf_computersystem.SystemType),
			ModelNumber:  rf_computersystem.Model,
			SerialNumber: rf_computersystem.SerialNumber,
			SerialConsole: models.SerialConsole{
				IPMI: models.SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.IPMI.ServiceEnabled,
				},
				SSH: models.SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.SSH.ServiceEnabled,
				},
				Telnet: models.SerialConsoleConfig{
					Enabled: rf_computersystem.SerialConsole.Telnet.ServiceEnabled,
				},
			},
			BiosVersion: rf_computersystem.BiosVersion,
			Links: models.Links{
				Managers: managerLinks,
				Chassis:  chassisLinks,
			},
			Power: models.Power{
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
			ethernetinterface := models.EthernetInterface{
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

			var networkAdapter models.NetworkAdapter
			if rf_networkAdapter != nil {
				networkAdapter = models.NetworkAdapter{
					URI:          baseURI + rf_networkAdapter.ODataID,
					Name:         rf_networkAdapter.Name,
					Manufacturer: rf_networkAdapter.Manufacturer,
					Model:        rf_networkAdapter.Model,
					Serial:       rf_networkAdapter.SerialNumber,
					Description:  rf_networkAdapter.Description,
				}
			}

			networkInterface := models.NetworkInterface{
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
func walkManagers(rf_managers []*schemas.Manager, baseURI string) ([]models.Manager, error) {
	var managers []models.Manager
	for _, rf_manager := range rf_managers {
		rf_ethernetinterfaces, err := rf_manager.EthernetInterfaces()
		if err != nil {
			log.Error().Err(err).Msg("failed to get ethernet interfaces from manager")
			return managers, err
		}
		var ethernet_interfaces []models.EthernetInterface
		for _, rf_ethernetinterface := range rf_ethernetinterfaces {
			if len(rf_ethernetinterface.IPv4Addresses) <= 0 {
				continue
			}
			ethernet_interfaces = append(ethernet_interfaces, models.EthernetInterface{
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

		managers = append(managers, models.Manager{
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

func merge(systems map[string]*models.InventoryDetail, newSystems []models.InventoryDetail) map[string]*models.InventoryDetail {
	// add and replace values in systems with values from newSystems
	for _, system := range newSystems {
		systems[system.URI] = &system
	}
	return systems
}
