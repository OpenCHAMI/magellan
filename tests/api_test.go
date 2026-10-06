//go:build integration

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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flag"

	"github.com/openchami/magellan/internal/util"
	magellan "github.com/openchami/magellan/pkg"
	"github.com/openchami/magellan/pkg/client"
)

var (
	exePath = flag.String("exe", "../magellan", "path to 'magellan' binary executable")
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
		cache   = filepath.Join(t.TempDir(), "assets.db")
	)

	// say what test we're starting
	fmt.Printf("[%s] Starting test...", t.Name())

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
	command = []string{"scan", "https://127.0.0.1", "--port", "5000", "--log-level", "debug", "--insecure", "--cache", cache, "--output-format", "json"}
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()

	// show output and error of test
	fmt.Printf("[%s INFO] %s\n [%s ERR] %s\n",
		t.Name(), bufout.String(),
		t.Name(), buferr.String(),
	)

	if err != nil {
		t.Fatalf("failed to run 'scan' command: %v", err)
	}

	var scanned []magellan.RemoteAsset
	if err := json.Unmarshal(bufout.Bytes(), &scanned); err != nil {
		t.Fatalf("failed to decode scan output: %v\n%s", err, bufout.String())
	}
	if len(scanned) != 1 || scanned[0].Port != 5000 || !scanned[0].State {
		t.Fatalf("unexpected scan result: %#v", scanned)
	}

	// try and run a "collect" with the emulator

	bufout.Reset()
	buferr.Reset()
	command = []string{"collect", "--cache", cache, "--username", "root", "--password", "root_password", "--log-level", "debug", "--insecure", "--show-output", "--output-format", "json"}
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()

	// show output and error of test
	fmt.Printf("[%s INFO] %s\n [%s ERR] %s\n",
		t.Name(), bufout.String(),
		t.Name(), buferr.String(),
	)

	if err != nil {
		t.Fatalf("failed to run 'collect' command: %v", err)
	}

	var inventory []map[string]any
	if err := json.Unmarshal(bufout.Bytes(), &inventory); err != nil {
		t.Fatalf("failed to decode collect output: %v\n%s", err, bufout.String())
	}
	if len(inventory) == 0 {
		t.Fatal("expected collect to return inventory")
	}
	if systems, ok := inventory[0]["Systems"].([]any); !ok || len(systems) == 0 {
		t.Fatalf("expected collected systems, got %#v", inventory[0]["Systems"])
	}

	// say what test we're completing
	fmt.Printf("[%s] Test complete.", t.Name())

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

	// say what test we're starting
	fmt.Printf("[%s] Starting test...", t.Name())

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
	command = strings.Split("crawl --username root --password root_password --insecure https://127.0.0.1:5000 --show-output", " ")
	cmd = exec.Command(path, command...)
	cmd.Stdout = &bufout
	cmd.Stderr = &buferr
	err = cmd.Run()

	// show output and error of test
	fmt.Printf("[%s INFO] %s\n [%s ERR] %s\n",
		t.Name(), bufout.String(),
		t.Name(), buferr.String(),
	)

	if err != nil {
		t.Fatalf("failed to run 'crawl' command: %v", err)
	}

	// make sure that the output is not empty
	if len(bufout.Bytes()) <= 0 {
		t.Fatalf("expected the 'crawl' output to not be empty")
	}

	// say what test we're completing
	fmt.Printf("[%s] Test complete.", t.Name())
}

// waitUntilEmulatorIsReady() polls with
func waitUntilEmulatorIsReady() error {
	var (
		interval   = time.Second * 2
		timeout    = time.Second * 30
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
