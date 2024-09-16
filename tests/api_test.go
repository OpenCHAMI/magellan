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
	"os/exec"
	"testing"

	"flag"

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

	// set up the test

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
	// set up temporary directory
	cmd = exec.Command("bash", "-c", fmt.Sprintf("%s list", *exePath))
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run 'list' command: %v", err)
	}
}

func TestGofishFunctions(t *testing.T) {
	// TODO: add test that checks certain gofish function output to make sure
	// gofish's output isn't changing spontaneously and remains predictable
}

func TestGenerateHosts(t *testing.T) {
	// TODO: add test to generate hosts using a collection of subnets/masks
	t.Run("generate-hosts.1", func(t *testing.T) {
		var (
			subnet     = "172.16.0.0"
			subnetMask = &net.IPMask{255, 255, 255, 0}
			ports      = []int{443}
			scheme     = "https"
			hosts      = [][]string{}
		)
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)
	})

	t.Run("generate-hosts.2", func(t *testing.T) {
		var (
			subnet     = "127.0.0.1"
			subnetMask = &net.IPMask{255, 255, 255, 0}
			ports      = []int{443, 5000}
			scheme     = "https"
			hosts      = [][]string{}
		)
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)
	})

}
