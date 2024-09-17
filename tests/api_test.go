// This file contains generic tests used to confirm expected behaviors of the
// builtin APIs. This is to guarantee that our functions work as expected
// regardless of the hardware being used such as testing the `scan`, and `collect`
// functionality and `gofish` library and asserting expected outputs.
//
// These tests are meant to be ran with the emulator included in the project.
// Make sure the emulator is running before running the tests.
package tests

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"flag"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/OpenCHAMI/magellan/pkg/client"
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
	exePath = flag.String("exe", "./magellan", "path to 'magellan' binary executable")
	emuPath = flag.String("emu", "./emulator/setup.sh", "path to emulator 'setup.sh' script")
)

func runEmulator() {}

func TestScanAndCollect(t *testing.T) {
	var (
		err     error
		emuErr  error
		output  []byte
		tempDir = t.TempDir()
		command string
	)

	// try and start the emulator in the background if arg passed
	if *emuPath != "" {
		t.Parallel()
		t.Run("emulator", func(t *testing.T) {
			_, emuErr = exec.Command("bash", "-c", *emuPath).CombinedOutput()
			if emuErr != nil {
				t.Fatalf("failed to start emulator: %v", emuErr)
			}
		})
	}

	// try and run a "scan" with the emulator
	command = fmt.Sprintf("%s scan --subnet 127.0.0.1 --subnet-mask 255.255.255.0 --cache %s", exePath, tempDir)
	output, err = exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'scan' command: %v", err)
	}

	// make sure that the expected output is not empty
	if len(output) <= 0 {
		t.Fatalf("expected the 'scan' output to not be empty")
	}

	// try and run a "collect" with the emulator
	command = fmt.Sprintf("%s collect --username root --password root_password --cache %s", exePath, tempDir)
	output, err = exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'collect' command: %v", err)
	}

	// make sure that the output is not empty
	if len(output) <= 0 {
		t.Fatalf("expected the 'collect' output to not be empty")
	}

	// TODO: check for at least one System/EthernetInterface that we know should exist
}

func TestCrawlCommand(t *testing.T) {
	var (
		err     error
		emuErr  error
		output  []byte
		command string
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed to start emulator: %v", err)
	}

	// try and start the emulator in the background if arg passed
	if *emuPath != "" {
		t.Parallel()
		t.Run("emulator", func(t *testing.T) {
			_, emuErr = exec.Command("bash", "-c", *emuPath).CombinedOutput()
			if emuErr != nil {
				t.Fatalf("failed to start emulator: %v", emuErr)
			}
		})
	}

	// try and run a "collect" with the emulator
	command = fmt.Sprintf("%s crawl --username root --password root_password -i", exePath)
	output, err = exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'crawl' command: %v", err)
	}

	// make sure that the output is not empty
	if len(output) <= 0 {
		t.Fatalf("expected the 'crawl' output to not be empty")
	}

}

func TestListCommand(t *testing.T) {
	// TODO: need magellan binary to test command
	var (
		cmd    *exec.Cmd
		err    error
		output []byte
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed to start emulator: %v", err)
	}

	// set up temporary directory
	cmd = exec.Command("bash", "-c", fmt.Sprintf("%s list", *exePath))
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'list' command: %v", err)
	}

	// make sure that the output is not empty
	if len(output) <= 0 {
		t.Fatalf("expected the 'list' output to not be empty")
	}

}

func TestUpdateCommand(t *testing.T) {
	// TODO: add test that does a Redfish simple update checking it success and
	// failure points
	var (
		cmd    *exec.Cmd
		err    error
		output []byte
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed to start emulator: %v", err)
	}

	// set up temporary directory
	cmd = exec.Command("bash", "-c", fmt.Sprintf("%s list", *exePath))
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'list' command: %v", err)
	}

	// make sure that the output is not empty
	if len(output) <= 0 {
		t.Fatalf("expected the 'list' output to not be empty")
	}
}

func TestGofishFunctions(t *testing.T) {
	// TODO: add test that checks certain gofish function output to make sure
	// gofish's output isn't changing spontaneously and remains predictable
}

// TestGenerateHosts() tests creating a collection of hosts by changing arguments
// and calling GenerateHostsWithSubnet().
func TestGenerateHosts(t *testing.T) {
	// TODO: add test to generate hosts using a collection of subnets/masks
	var (
		subnet     = "127.0.0.1"
		subnetMask = &net.IPMask{255, 255, 255, 0}
		ports      = []int{443}
		scheme     = "https"
		hosts      = [][]string{}
	)
	t.Run("generate-hosts", func(t *testing.T) {
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)

		// check for at least one host to be generated
		if len(hosts) <= 0 {
			t.Fatalf("expected at least one host to be generated for subnet %s", subnet)
		}
	})

	t.Run("generate-hosts-with-multiple-ports", func(t *testing.T) {
		ports = []int{443, 5000}
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)

		// check for at least one host to be generated
		if len(hosts) <= 0 {
			t.Fatalf("expected at least one host to be generated for subnet %s", subnet)
		}
	})

	t.Run("generate-hosts-with-subnet-mask", func(t *testing.T) {
		subnetMask = &net.IPMask{255, 255, 125, 0}
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)

		// check for at least one host to be generated
		if len(hosts) <= 0 {
			t.Fatalf("expected at least one host to be generated for subnet %s", subnet)
		}
	})

}

// waitUntilEmulatorIsReady() polls with
func waitUntilEmulatorIsReady() error {
	var (
		interval   = time.Second * 5
		timeout    = time.Second * 60
		testClient = &http.Client{}
		body       client.HTTPBody
		header     client.HTTPHeader
		err        error
	)
	err = util.CheckUntil(interval, timeout, func() (bool, error) {
		// send request to host until we get expected response
		res, _, err := client.MakeRequest(testClient, "http://127.0.0.1", http.MethodPost, body, header)
		if err != nil {
			return false, fmt.Errorf("failed to start emulator: %w", err)
		}
		if res == nil {
			return false, fmt.Errorf("response returned nil")
		}
		if res.StatusCode == http.StatusOK {
			return true, nil
		}
		return false, nil
	})
	return err
}
