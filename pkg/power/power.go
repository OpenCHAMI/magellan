package power

import (
	"fmt"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/crawler"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/schemas"
)

type CrawlableNode struct {
	ClusterID  string
	ConnConfig crawler.CrawlerConfig
	NodeID     string
}
type PowerInfo struct {
	ClusterID string
	State     schemas.PowerState
}

// Hold onto the current set of open clients, so we don't continually have to log into and out of BMCs
var savedClients map[string]*gofish.APIClient

// ResetComputerSystem connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and retrieves the list of supported reset types for the target ComputerSystem.
//
// Parameters:
//   - node: A CrawlableNode struct containing the node's xname, index within the BMC, and a CrawlerConfig to connect to the BMC.
//
// Returns:
//   - []schemas.ResetType: a slice of Redfish reset types supported on the node.
//   - error: An error object if any error occurs during the connection or reset process.
func GetResetTypes(node CrawlableNode) ([]schemas.ResetType, error) {
	log.Debug().Msgf("polling %s for reset types", node.ConnConfig.URI)

	// Obtain an active client
	client, err := GetBMCSession(node.ConnConfig)
	if err != nil {
		return nil, err
	}

	// Determine reset types for the target computer system
	rf_systems, err := client.GetService().Systems()
	if err != nil {
		return nil, err
	}
	var system *schemas.ComputerSystem
	for i := range rf_systems {
		if rf_systems[i].ID == node.NodeID {
			system = rf_systems[i]
			break
		}
	}

	resetTypes, err := system.GetSupportedResetTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get supported reset types for: %v", err)
	}
	return resetTypes, nil
}

// PollBMCPowerStates connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and retrieves the current power state for each ComputerSystem in each Chassis.
//
// Parameters:
//   - node: A CrawlableNode struct containing the target node's xname, index within the BMC, and a crawler.CrawlerConfig struct.
//
// Returns:
//   - schemas.PowerState: The current power state of the node. (Custom string subtype)
//   - error: An error object if any error occurs during the connection or retrieval process.
func GetPowerState(node CrawlableNode) (schemas.PowerState, error) {
	log.Debug().Msgf("polling %s for power states", node.ConnConfig.URI)

	// Obtain an active client
	client, err := GetBMCSession(node.ConnConfig)
	if err != nil {
		return "", err
	}

	// Determine power details for the target computer system
	rf_systems, err := client.GetService().Systems()
	if err != nil {
		return "", err
	}
	var system *schemas.ComputerSystem
	for i := range rf_systems {
		if rf_systems[i].ID == node.NodeID {
			system = rf_systems[i]
			break
		}
	}
	return system.PowerState, nil
}

// ResetComputerSystem connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and issues a reset of the specified type to a particular computer system.
//
// Parameters:
//   - node: A CrawlableNode struct containing the node's xname, index within the BMC, and a CrawlerConfig to connect to the BMC.
//   - resetType: A schemas.ResetType parameter, specifying the manner in which the target ComputerSystem should be reset.
//
// Returns:
//   - error: An error object if any error occurs during the connection or reset process.
func ResetComputerSystem(node CrawlableNode, resetType schemas.ResetType) (*schemas.TaskMonitorInfo, error) {
	log.Debug().Msgf("resetting computer system %s: %s", node.ClusterID, resetType)

	client, err := bmc.ConnectWithCredentials(node.ConnConfig.URI, node.ConnConfig.CredentialStore, node.ConnConfig.Insecure, node.ConnConfig.CACertPath)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()
	log.Debug().Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	// Select the relevant ComputerSystem
	rf_systems, err := rf_service.Systems()
	if err != nil {
		return nil, err
	}
	var rf_compsys *schemas.ComputerSystem
	for i := range rf_systems {
		if rf_systems[i].ID == node.NodeID {
			rf_compsys = rf_systems[i]
			break
		}
	}

	// Reset the system
	return rf_compsys.Reset(resetType)
}

// GetBMCSession returns an already-active gofish BMC client, creating a new one if necessary.
// This facilitates keeping the clients open for efficiency.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//
// Returns: none.
func GetBMCSession(config crawler.CrawlerConfig) (*gofish.APIClient, error) {
	client, exists := savedClients[config.URI]
	if exists {
		log.Debug().Msgf("found existing client for %s", config.URI)
	} else {
		if savedClients == nil {
			savedClients = make(map[string]*gofish.APIClient)
		}
		var err error
		client, err = bmc.ConnectWithCredentials(config.URI, config.CredentialStore, config.Insecure, config.CACertPath)
		if err != nil {
			return nil, err
		}
		log.Debug().Msgf("created new client for %s", config.URI)
		savedClients[config.URI] = client
	}
	return client, nil
}

// LogoutBMCSessions logs out all active gofish BMC clients, which we normally like to keep open for efficiency.
// Logging out should be done as a post-execution cleanup step.
//
// Parameters: none.
//
// Returns: none.
func LogoutBMCSessions() {
	for uri, client := range savedClients {
		log.Debug().Msgf("logging out client for %s", uri)
		client.Logout()
	}
}
