// This file contains a series of tests that are meant to ensure correct
// Redfish behaviors and responses across different Refish implementations
// and are expected to be ran with various hardware and firmware to test
// compatibility with the tool. These tests are meant to be used as a way
// to pinpoint exactly where an issue is occurring in a more predictable
// and reproducible manner.
package tests

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
)

var (
	host     = flag.String("host", "localhost", "set the BMC host")
	username = flag.String("username", "", "set the BMC username used for the tests")
	password = flag.String("password", "", "set the BMC password used for the tests")
)

func checkResponse(res *http.Response, b []byte) error {
	// test for a 200 response code here
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("expected response code to return status code 200")
	}

	// make sure the response body is not empty
	if len(b) <= 0 {
		return fmt.Errorf("expected response body to not be empty")
	}

	// make sure the response body is in a valid JSON format
	if json.Valid(b) {
		return fmt.Errorf("expected response body to be valid JSON")
	}
	return nil
}

// Simple test to fetch the base Redfish URL and assert a 200 OK response.
func TestRedfishV1ServiceRootAvailability(t *testing.T) {
	var (
		url     = fmt.Sprintf("%s/redfish/v1", *host)
		body    = []byte{}
		headers = map[string]string{}
	)
	res, b, err := client.MakeRequest(nil, url, http.MethodGet, body, headers)
	if err != nil {
		t.Fatalf("failed to make request to BMC node: %v", err)
	}

	err = checkResponse(res, b)
	if err != nil {
		t.Fatalf("failed to check response for redfish service root: %v", err)
	}

}

// Simple test to ensure an expected Redfish version minimum requirement.
func TestRedfishV1Version(t *testing.T) {
	var (
		url     string            = fmt.Sprintf("%s/redfish/v1", *host)
		body    client.HTTPBody   = []byte{}
		headers client.HTTPHeader = map[string]string{}
		err     error
	)

	res, b, err := client.MakeRequest(nil, url, http.MethodGet, body, headers)
	if err != nil {
		t.Fatalf("failed to make request to BMC node: %v", err)
	}
	err = checkResponse(res, b)
	if err != nil {
		t.Fatalf("failed to check response for redfish version: %v", err)
	}
}

// Crawls a BMC node and checks that we're able to query certain properties
// that we need for Magellan to run correctly. This test differs from the
// `TestCrawlCommand` testing function as it is not checking specifically
// for functionality.
func TestExpectedOutput(t *testing.T) {
	// make sure what have a valid host
	if host == nil {
		t.Fatal("invalid host (host is nil)")
	}

	systems, err := crawler.CrawlBMC(
		crawler.CrawlerConfig{
			URI:      *host,
			Username: *username,
			Password: *password,
			Insecure: true,
		},
	)

	if err != nil {
		t.Fatalf("failed to crawl BMC: %v", err)
	}

	// check that we got results in systems
	if len(systems) <= 0 {
		t.Fatal("no systems found")
	}

	// check that we're getting EthernetInterfaces and NetworkInterfaces
	for _, system := range systems {
		// check that we have at least one CPU for each system
		if system.ProcessorCount <= 0 {
			t.Errorf("no processors found")
		}
		// we expect each system to have at least one of each interface
		if len(system.EthernetInterfaces) <= 0 {
			t.Errorf("no ethernet interfaces found for system '%s'", system.Name)
		}
		if len(system.NetworkInterfaces) <= 0 {
			t.Errorf("no network interfaces found for system '%s'", system.Name)
		}
	}
}
