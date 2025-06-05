package jaws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/pdu"
)

type CrawlerConfig struct {
	URI      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// CrawlPDU connects to a single JAWS PDU and collects its inventory.
func CrawlPDU(config CrawlerConfig) (*pdu.PDUInventory, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}
	_ = client

	inventory := &pdu.PDUInventory{
		Hostname: config.URI,
	}

	// 1. Get System Info
	// Should call /jaws/config/info/system
	// Create a temporary struct to unmarshal the response
	// and then populate PDU inventory, like is done in CSM.

	// 2. Get Outlet Status
	// Should call /jaws/outlet/status or similar endpoint
	// It will return a list of outlets to parse

	fmt.Printf("Crawling JAWS PDU at %s...\n", config.URI)

	return inventory, nil
}

/*
func getSystemInfo(client *http.Client, config CrawlerConfig) (*SystemInfo, error) {
    // GET to /jaws/config/info/system
}

func getOutletStatus(client *http.Client, config CrawlerConfig) ([]pdu.PDUOutlet, error) {
    // GET to /jaws/outlet/status
}
*/
