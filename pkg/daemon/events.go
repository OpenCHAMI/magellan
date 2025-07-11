package daemon

import (
	"encoding/json"

	"github.com/OpenCHAMI/magellan/pkg/crawler"

	"github.com/rs/zerolog/log"
	"github.com/stmcginnis/gofish/redfish"
)

type Subscription struct {
	Destination      string
	RegistryPrefixes []string
	ResourceTypes    []string
	HttpHeaders      map[string]string
	Context          string
	Insecure         bool
}

type PowerInfo struct {
	Xname string
	State redfish.PowerState
}

// CreateBMCPowerSubscription connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and then creates event subscriptions for power state changes.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//   - sub: A Subscription struct containing the callback URI, registry prefixes, and other Redfish subscription details.
//
// Returns:
//   - string: The URI of the newly created Redfish event subscription.
//   - error: An error object if any error occurs during the connection or retrieval process.
func CreateBMCPowerSubscription(config crawler.CrawlerConfig, sub Subscription) (string, error) {
	log.Debug().Msgf("creating Redfish power subscription on %s", config.URI)

	client, err := crawler.GetBMCClient(config)
	if err != nil {
		return "", err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()
	log.Debug().Msgf("found ServiceRoot %s. Redfish Version %s", rf_service.ID, rf_service.RedfishVersion)

	// Obtain the EventService
	ev_service, err := rf_service.EventService()
	if err != nil {
		log.Error().Err(err).Msg("failed to get event service from ServiceRoot")
		return "", err
	}
	ev_json, _ := json.Marshal(ev_service)
	log.Debug().Msgf("found event service %s", ev_json)

	// Create actual event subscription
	sub_uri, err := ev_service.CreateEventSubscriptionInstance(
		sub.Destination,
		sub.RegistryPrefixes,
		sub.ResourceTypes,
		sub.HttpHeaders,
		redfish.RedfishEventDestinationProtocol,
		sub.Context,
		// Empty delivery retry policy and OEM content
		"", nil)
	if err != nil {
		return "", err
	}

	// Set VerifyCertificate to false, if the subscription is to be insecure
	if sub.Insecure {
		ev_destination, err := redfish.GetEventDestination(client, sub_uri)
		if err != nil {
			return sub_uri, err
		}
		ev_destination.VerifyCertificate = false
		err = ev_destination.Update()
	}

	return sub_uri, err
}

// DeleteBMCPowerSubscription connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and then creates event subscriptions for power state changes.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//   - sub: The Redfish URI for an existing subscription, to be deleted.
//
// Returns:
//   - error: An error object if any error occurs during the connection or retrieval process.
func DeleteBMCPowerSubscription(config crawler.CrawlerConfig, subUri string) error {
	log.Debug().Msgf("deleting Redfish power subscription %s", subUri)

	client, err := crawler.GetBMCClient(config)
	if err != nil {
		return err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()

	// Obtain the EventService
	ev_service, err := rf_service.EventService()
	if err != nil {
		log.Error().Err(err).Msg("failed to get event service from ServiceRoot")
		return err
	}
	ev_json, _ := json.Marshal(ev_service)
	log.Debug().Msgf("found event service %s", ev_json)

	return ev_service.DeleteEventSubscription(subUri)
}

// PollBMCPowerStates connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and retrieves the current power state for each ComputerSystem in each Chassis.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//
// Returns:
//   - []PowerInfo: An array of power information structs, containing a computer system ID (xname) and Redfish PowerState.
//   - error: An error object if any error occurs during the connection or retrieval process.
func PollBMCPowerStates(config crawler.CrawlerConfig) ([]PowerInfo, error) {
	log.Debug().Msgf("polling BMC %s for power states", config.URI)

	// TODO: Factor this out, so we can cache the result and poll more efficiently
	client, err := crawler.GetBMCClient(config)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	// Obtain the ServiceRoot
	rf_service := client.GetService()

	// Obtain the Chassis list
	rf_chassis, err := rf_service.Chassis()
	if err != nil {
		log.Error().Err(err).Msg("failed to get chassis list from ServiceRoot")
		return nil, err
	}
	ch_json, _ := json.Marshal(rf_chassis)
	log.Debug().Msgf("found chassis list %s", ch_json)

	// Determine power details for each computer system in each chassis
	var powerInfo []PowerInfo
	for _, chassis := range rf_chassis {
		rf_systems, err := chassis.ComputerSystems()
		if err != nil {
			log.Warn().Err(err).Msgf("failed to get computer systems from chassis %s", chassis.ID)
			continue
		}
		for _, system := range rf_systems {
			powerInfo = append(powerInfo, PowerInfo{
				// FIXME: This is not, in fact, an xname. BMCs don't know their own, so we'll have to query SMD, probably?
				Xname: system.Name,
				State: system.PowerState,
			})
		}
	}
	return powerInfo, nil
}
