package jaws

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/pdu"
	"github.com/rs/zerolog/log"
)

type CrawlerConfig struct {
	URI      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// JawsOutlet represents the structure of a single outlet object
type JawsOutlet struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	Current     float32 `json:"current"`
	Voltage     float32 `json:"voltage"`
	ActivePower int     `json:"active_power"`
}

// CrawlPDU connects to a single JAWS PDU and collects its full inventory.
func CrawlPDU(config CrawlerConfig) (*pdu.PDUInventory, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Insecure},
	}
	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	inventory := &pdu.PDUInventory{
		Hostname: config.URI,
	}

	targetURL := fmt.Sprintf("https://%s/jaws/monitor/outlets", config.URI)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to create new HTTP request for outlets")
		return nil, err
	}
	req.SetBasicAuth(config.Username, config.Password)

	log.Debug().Msgf("querying JAWS endpoint: %s", targetURL)
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msgf("failed to execute request to JAWS outlets endpoint %s", targetURL)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("received non-200 status code from outlets endpoint: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		log.Error().Err(err).Str("url", targetURL).Msg("bad response from PDU")
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read outlets response body")
		return nil, err
	}
	log.Debug().RawJSON("response_body", body).Msg("received response from JAWS outlets")

	var rawOutlets []JawsOutlet
	if err := json.Unmarshal(body, &rawOutlets); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal JAWS outlet data")
		return nil, err
	}

	for _, rawOutlet := range rawOutlets {
		outlet := pdu.PDUOutlet{
			ID:         rawOutlet.ID,
			Name:       rawOutlet.Name,
			PowerState: rawOutlet.State,
		}
		inventory.Outlets = append(inventory.Outlets, outlet)
	}

	log.Info().Msgf("successfully collected inventory for %d outlets from %s", len(inventory.Outlets), config.URI)
	return inventory, nil
}
