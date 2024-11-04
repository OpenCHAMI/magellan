// This file contains generic tests used to confirm expected behaviors of the
// builtin APIs. This is to guarantee that our functions work as expected
// regardless of the hardware being used such as testing the `scan`, and `collect`
// functionality and `gofish` library and asserting expected outputs.
//
// These tests are meant to be ran with the emulator included in the project.
// Make sure the emulator is running before running the tests.
package tests

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flag"

	magellan "github.com/davidallendj/magellan/internal"
	"github.com/davidallendj/magellan/internal/util"
	"github.com/davidallendj/magellan/pkg/client"
	"github.com/rs/zerolog/log"
)

var (
	exePath = flag.String("exe", "../magellan", "path to 'magellan' binary executable")
	emuPath = flag.String("emu", "./emulator/setup.sh", "path to emulator 'setup.sh' script")
)

func TestScanAndCollect(t *testing.T) {
	var (
		err error
		// tempDir = t.TempDir()
		path    string
		command []string
		cwd     string
		cmd     *exec.Cmd
		bufout  bytes.Buffer
		buferr  bytes.Buffer
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed while waiting for emulator: %v", err)
	}

	// get the current working directory and print
	cwd, err = os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	fmt.Printf("cwd: %s\n", cwd)

	// path, err := exec.LookPath("dexdump")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// try and run a "scan" with the emulator
	// set up the emulator to run before test
	path, err = filepath.Abs(*exePath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	command = strings.Split("scan https://127.0.0.1 --port 5000 --verbose", " ")
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()
	fmt.Printf("out:\n%s\nerr:\n%s\n", bufout.String(), buferr.String())
	if err != nil {
		t.Fatalf("failed to run 'scan' command: %v", err)
	}

	// make sure that the expected output is not empty
	if len(buferr.Bytes()) <= 0 {
		t.Fatalf("expected the 'scan' output to not be empty")
	}

	// try and run a "collect" with the emulator

	command = strings.Split("collect --username root --password root_password --verbose", " ")
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()
	fmt.Printf("out:\n%s\nerr:\n%s\n", bufout.String(), buferr.String())
	if err != nil {
		t.Fatalf("failed to run 'collect' command: %v", err)
	}

	// make sure that the output is not empty
	if len(bufout.Bytes()) <= 0 {
		t.Fatalf("expected the 'collect' output to not be empty")
	}

	// TODO: check for at least one System/EthernetInterface that we know should exist
}

func TestCrawlCommand(t *testing.T) {
	var (
		err     error
		command []string
		cmd     *exec.Cmd
		bufout  bytes.Buffer
		buferr  bytes.Buffer
		path    string
	)

	// set up the emulator to run before test
	path, err = filepath.Abs(*exePath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	fmt.Printf("path: %s\n", path)
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed while waiting for emulator: %v", err)
	}

	// try and run a "collect" with the emulator
	command = strings.Split("crawl --username root --password root_password -i https://127.0.0.1:5000", " ")
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()
	fmt.Printf("out:\n%s\nerr:\n%s\n", bufout.String(), buferr.String())
	if err != nil {
		t.Fatalf("failed to run 'crawl' command: %v", err)
	}

	// err = cmd.Wait()
	// if err != nil {
	// 	t.Fatalf("failed to call 'wait' for crawl: %v", err)
	// }

	// make sure that the output is not empty
	if len(bufout.Bytes()) <= 0 {
		t.Fatalf("expected the 'crawl' output to not be empty")
	}

}

func TestListCommand(t *testing.T) {
	var (
		err error
		cmd *exec.Cmd
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed while waiting for emulator: %v", err)
	}

	// set up temporary directory
	cmd = exec.Command("bash", "-c", fmt.Sprintf("%s list", *exePath))
	err = cmd.Start()
	if err != nil {
		t.Fatalf("failed to run 'list' command: %v", err)
	}
	// NOTE: the output of `list` can be empty if no scan has been performed

}

func TestUpdateCommand(t *testing.T) {
	// TODO: add test that does a Redfish simple update checking it success and
	// failure points
	var (
		cmd *exec.Cmd
		err error
	)

	// set up the emulator to run before test
	err = waitUntilEmulatorIsReady()
	if err != nil {
		t.Fatalf("failed while waiting for emulator: %v", err)
	}

	// set up temporary directory
	cmd = exec.Command("bash", "-c", fmt.Sprintf("%s update", *exePath))
	err = cmd.Start()
	if err != nil {
		t.Fatalf("failed to run 'update' command: %v", err)
	}

}

func TestGofishFunctions(t *testing.T) {
	// TODO: add test that checks certain gofish function output to make sure
	// gofish's output isn't changing spontaneously and remains predictable
}

// TestGenerateHosts() tests creating a collection of hosts by changing arguments
// and calling GenerateHostsWithSubnet().
func TestGenerateHosts(t *testing.T) {
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
		subnetMask = &net.IPMask{255, 255, 0, 0}
		hosts = magellan.GenerateHostsWithSubnet(subnet, subnetMask, ports, scheme)

		// check for at least one host to be generated
		if len(hosts) <= 0 {
			t.Fatalf("expected at least one host to be generated for subnet %s", subnet)
		}
	})

}

func startEmulatorInBackground(path string) (int, error) {
	// try and start the emulator in the background if arg passed
	var (
		cmd *exec.Cmd
		err error
	)
	if path != "" {
		cmd = exec.Command("bash", "-c", path)
		err = cmd.Start()
		if err != nil {
			return -1, fmt.Errorf("failed while executing emulator startup script: %v", err)
		}
	} else {
		return -1, fmt.Errorf("path to emulator start up script is required")
	}
	return cmd.Process.Pid, nil
}

// waitUntilEmulatorIsReady() polls with
func waitUntilEmulatorIsReady() error {
	var (
		interval   = time.Second * 2
		timeout    = time.Second * 6
		testClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		body   client.HTTPBody
		header client.HTTPHeader
		err    error
	)
	err = util.CheckUntil(interval, timeout, func() (bool, error) {
		// send request to host until we get expected response
		res, _, err := client.MakeRequest(testClient, "https://127.0.0.1:5000/redfish/v1/", http.MethodGet, body, header)
		if err != nil {
			return false, fmt.Errorf("failed to make request to emulator: %w", err)
		}
		if res == nil {
			return false, fmt.Errorf("invalid response from emulator (response is nil)")
		}
		if res.StatusCode == http.StatusOK {
			return true, nil
		} else {
			return false, fmt.Errorf("unexpected status code %d", res.StatusCode)
		}

	})
	return err
}

func init() {
	var (
		cwd string
		err error
	)
	// get the current working directory
	cwd, err = os.Getwd()
	if err != nil {
		log.Error().Err(err).Msg("failed to get working directory")
	}
	fmt.Printf("cwd: %s\n", cwd)

	// start emulator in the background before running tests
	pid, err := startEmulatorInBackground(*emuPath)
	if err != nil {
		log.Error().Err(err).Msg("failed to start emulator in background")
		os.Exit(1)
	}
	_ = pid
}
