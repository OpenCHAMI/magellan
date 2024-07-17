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
	"github.com/OpenCHAMI/magellan/internal/log"
	"github.com/sirupsen/logrus"
)

func TestScanAndCollect(t *testing.T) {
	var (
		hosts = []string{"http://127.0.0.1"}
		ports = []int{5000}
		l     = log.NewLogger(logrus.New(), logrus.DebugLevel)
	)
	// do a scan on the emulator cluster with probing disabled and check results
	results := magellan.ScanForAssets(hosts, ports, 1, 30, true, false)
	if len(results) <= 0 {
		t.Fatal("expected to find at least one BMC node, but found none")
	}
	// do a scan on the emulator cluster with probing enabled
	results = magellan.ScanForAssets(hosts, ports, 1, 30, false, false)
	if len(results) <= 0 {
		t.Fatal("expected to find at least one BMC node, but found none")
	}

	// do a collect on the emulator cluster to collect Redfish info
	magellan.CollectAll(results)
}

func TestCrawlCommand(t *testing.T) {

}

func TestListCommand(t *testing.T) {

}

func TestUpdateCommand(t *testing.T) {

}

func TestGofishFunctions(t *testing.T) {

}
