// This file contains generic tests used to confirm expected behaviors of the
// builtin APIs. This is to guarantee that our functions work as expected
// regardless of the hardware being used such as testing the `scan`, and `collect`
// functionality and `gofish` library and asserting expected outputs.
//
// These tests are meant to be ran with the emulator included in the project.
// Make sure the emulator is running before running the tests.
package tests

import (
	"testing"

	magellan "github.com/OpenCHAMI/magellan/internal"
)

var (
	scanParams = &magellan.ScanParams{
		TargetHosts: [][]string{
			[]string{
				"http://127.0.0.1:443",
				"http://127.0.0.1:5000",
			},
		},
		Scheme:         "https",
		Protocol:       "tcp",
		Concurrency:    1,
		Timeout:        30,
		DisableProbing: false,
		Verbose:        false,
	}
)

func TestScanAndCollect(t *testing.T) {
	// do a scan on the emulator cluster with probing disabled and check results
	results := magellan.ScanForAssets(scanParams)
	if len(results) <= 0 {
		t.Fatal("expected to find at least one BMC node, but found none")
	}
	// do a scan on the emulator cluster with probing enabled
	results = magellan.ScanForAssets(scanParams)
	if len(results) <= 0 {
		t.Fatal("expected to find at least one BMC node, but found none")
	}

	// do a collect on the emulator cluster to collect Redfish info
	magellan.CollectInventory(&results, &magellan.CollectParams{})
}

func TestCrawlCommand(t *testing.T) {
	// TODO: add test to check the crawl command's behavior
}

func TestListCommand(t *testing.T) {
	// TODO: add test to check the list command's output
}

func TestUpdateCommand(t *testing.T) {
	// TODO: add test that does a Redfish simple update checking it success and
	// failure points
}

func TestGofishFunctions(t *testing.T) {
	// TODO: add test that checks certain gofish function output to make sure
	// gofish's output isn't changing spontaneously and remains predictable
}

func TestGenerateHosts(t *testing.T) {
	// TODO: add test to generate hosts using a collection of subnets/masks
}
