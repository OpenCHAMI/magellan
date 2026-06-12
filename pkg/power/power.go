package power

import (
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

	// Obtain an active (cached) vendor-aware client
	client, err := bmc.DefaultManager.CachedClient(node.ConnConfig)
	if err != nil {
		return nil, err
	}
	return client.GetResetTypes(node.NodeID)
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

	// Obtain an active (cached) vendor-aware client
	client, err := bmc.DefaultManager.CachedClient(node.ConnConfig)
	if err != nil {
		return "", err
	}
	return client.GetPowerState(node.NodeID)
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
func ResetComputerSystem(node CrawlableNode, resetType schemas.ResetType) error {
	log.Debug().Msgf("resetting computer system %s: %s", node.ClusterID, resetType)

	// Use a fresh (uncached) vendor-aware client and log out when done.
	client, err := bmc.DefaultManager.Client(node.ConnConfig)
	if err != nil {
		return err
	}
	defer client.Logout()

	return client.Reset(node.NodeID, resetType)
}

// GetBMCSession returns an already-active gofish BMC client, creating a new one if necessary.
// This facilitates keeping the clients open for efficiency.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//
// Returns: none.
func GetBMCSession(config crawler.CrawlerConfig) (*gofish.APIClient, error) {
	client, err := bmc.DefaultManager.CachedClient(config)
	if err != nil {
		return nil, err
	}
	return client.Gofish(), nil
}

// LogoutBMCSessions logs out all active gofish BMC clients, which we normally like to keep open for efficiency.
// Logging out should be done as a post-execution cleanup step.
//
// Parameters: none.
//
// Returns: none.
func LogoutBMCSessions() {
	bmc.DefaultManager.LogoutAll()
}
