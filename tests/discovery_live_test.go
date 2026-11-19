//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openchami/magellan/internal/format"
	magellan "github.com/openchami/magellan/pkg"
	"github.com/openchami/magellan/pkg/secrets"
)

func TestLiveDiscovery(t *testing.T) {
	endpoint := os.Getenv("MAGELLAN_TEST_BMC_URL")
	username, password := os.Getenv("COLLECT_USERNAME"), os.Getenv("COLLECT_PASSWORD")
	if endpoint == "" || username == "" || password == "" {
		t.Skip("set MAGELLAN_TEST_BMC_URL, COLLECT_USERNAME, and COLLECT_PASSWORD to use the real lab BMC")
	}
	const invalidPassword = "invalid-discovery-test-password"
	const secretMarker = "discovery-test-secret-marker"
	work := t.TempDir()
	binary := filepath.Join(work, "magellan")
	build := exec.Command("go", "build", "-o", binary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	run := func(t *testing.T, wantCode int, wantDiagnostic, bmcPassword, token string, input []byte, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"--log-level", "trace"}, args...)...)
		cmd.Env = append(os.Environ(), "COLLECT_PASSWORD="+bmcPassword, "ACCESS_TOKEN="+token, "NO_PROXY=*")
		cmd.Stdin = bytes.NewReader(input)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		for _, secret := range []string{password, invalidPassword, secretMarker, token} {
			if secret != "" && strings.Contains(stderr.String(), secret) {
				t.Fatal("diagnostic exposed a secret")
			}
		}
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("execute: %v", err)
			}
			code = exit.ExitCode()
		}
		if code != wantCode || !strings.Contains(stderr.String(), wantDiagnostic) {
			t.Fatalf("exit = %d, want %d and diagnostic %q; stderr: %s", code, wantCode, wantDiagnostic, stderr.String())
		}
		if wantCode != 0 && len(output) != 0 {
			t.Fatal("failing command wrote to stdout")
		}
		return output
	}

	assetsJSON := run(t, 0, "", password, "", nil, "scan", endpoint, "--insecure", "--disable-cache", "-F", "json")
	collectArgs := []string{"collect", "--cache=", "--insecure", "--show-output"}
	inventoryJSON := run(t, 0, "", password, "", assetsJSON, collectArgs...)
	var inventory []map[string]any
	if err := json.Unmarshal(inventoryJSON, &inventory); err != nil || len(inventory) == 0 {
		t.Fatalf("successful collect did not return inventory: %v", err)
	}
	var assets []magellan.RemoteAsset
	if err := json.Unmarshal(assetsJSON, &assets); err != nil || len(assets) == 0 {
		t.Fatalf("successful scan did not return assets: %v", err)
	}
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := closed.Addr().(*net.TCPAddr).Port
	closedAddress := closed.Addr().String()
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("partial collection", func(t *testing.T) {
		partial := append(append([]magellan.RemoteAsset(nil), assets...), magellan.RemoteAsset{
			Host: "https://127.0.0.1", Port: closedPort, State: true,
		})
		input, err := json.Marshal(partial)
		if err != nil {
			t.Fatal(err)
		}
		run(t, 1, "failed to collect data", password, "", input, collectArgs...)
		got, err := magellan.CollectInventory(&partial, &magellan.CollectParams{
			Concurrency: 2, Insecure: true, OutputFormat: format.FORMAT_JSON,
			SecretStore: secrets.NewStaticStore(username, password),
		})
		if err == nil || !strings.Contains(err.Error(), "failed to collect") || len(got) != len(inventory) {
			t.Fatalf("expected retained inventory plus a collection error, got %d endpoints and %v", len(got), err)
		}
	})
	t.Run("invalid credentials", func(t *testing.T) {
		run(t, 1, "failed to collect data", invalidPassword, "", assetsJSON, collectArgs...)
	})
	t.Run("no responsive BMC", func(t *testing.T) {
		run(t, 1, "no responsive assets", password, "", nil, "scan", "https://"+closedAddress, "--disable-cache", "-F", "json")
	})
	t.Run("unreachable SMD", func(t *testing.T) {
		run(t, 1, "failed to make request", password, secretMarker, inventoryJSON, "send", "http://"+closedAddress)
	})
	t.Run("rejected inventory", func(t *testing.T) {
		destination := os.Getenv("MAGELLAN_TEST_REJECTING_SMD_URL")
		if destination == "" {
			t.Skip("set MAGELLAN_TEST_REJECTING_SMD_URL to a lab SMD that rejects inventory")
		}
		run(t, 1, "SMD rejected endpoint creation (HTTP 4", password, os.Getenv("ACCESS_TOKEN"),
			[]byte(`[{"ID":[],"DiscoveryTestMarker":"`+secretMarker+`"}]`), "send", destination)
	})
}