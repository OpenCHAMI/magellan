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

// CreateBMCPowerSubscription connects to a BMC (Baseboard Management Controller) using the provided configuration,
// retrieves the ServiceRoot, and then creates event subscriptions for power state changes.
//
// Parameters:
//   - config: A CrawlerConfig struct containing the URI, username, password, and other connection details.
//   - sub: A Subscription struct containing the callback URI, registry prefixes, and other Redfish subscription details.
//
// Returns:
//   - error: An error object if any error occurs during the connection or retrieval process.
func CreateBMCPowerSubscription(config crawler.CrawlerConfig, sub Subscription) (string, error) {
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
