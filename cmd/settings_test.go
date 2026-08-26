package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/magellan/internal/format"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func resetSettingsTestState(t *testing.T) {
	t.Helper()
	oldUsername, oldPassword := username, password
	oldSecretsFile, oldInsecure := secretsFile, insecure
	oldFormat, oldInputFormat := settingsFormat, settingsInputFormat
	oldInventory, oldCACert := settingsInventoryFile, settingsCACertPath
	oldPreserve := settingsPreserveConfig
	t.Cleanup(func() {
		username, password = oldUsername, oldPassword
		secretsFile, insecure = oldSecretsFile, oldInsecure
		settingsFormat, settingsInputFormat = oldFormat, oldInputFormat
		settingsInventoryFile, settingsCACertPath = oldInventory, oldCACert
		settingsPreserveConfig = oldPreserve
		viper.Reset()
		for _, command := range []commandFlagsForTest{
			{SettingsGetCmd, []string{"inventory-file", "input-format", "username", "password", "secrets-file", "insecure", "cacert", "output-format"}},
			{SettingsSetCmd, []string{"inventory-file", "input-format", "username", "password", "secrets-file", "insecure", "cacert"}},
			{SettingsResetCmd, []string{"inventory-file", "input-format", "username", "password", "secrets-file", "insecure", "cacert", "preserve-config"}},
		} {
			for _, name := range command.flags {
				if flag := command.command.Flags().Lookup(name); flag != nil {
					flag.Changed = false
				}
			}
		}
	})
	username, password, secretsFile = "", "", ""
	insecure = false
	settingsFormat = format.FORMAT_JSON
	settingsInputFormat = format.FORMAT_YAML
	settingsInventoryFile, settingsCACertPath, settingsPreserveConfig = "", "", ""
}

type commandFlagsForTest struct {
	command *cobra.Command
	flags   []string
}

func TestSettingsEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
		wantErr string
	}{
		{name: "IPv4", address: "192.0.2.10", want: "https://192.0.2.10"},
		{name: "hostname", address: "bmc01", want: "https://bmc01"},
		{name: "hostname and port", address: "bmc01:8443", want: "https://bmc01:8443"},
		{name: "IPv6", address: "2001:db8::10", want: "https://[2001:db8::10]"},
		{name: "existing URL", address: "http://127.0.0.1:8080/", want: "http://127.0.0.1:8080"},
		{name: "unsupported scheme", address: "ftp://bmc01", wantErr: "unsupported"},
		{name: "empty", address: " ", wantErr: "cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := settingsEndpoint(tt.address)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSettingsListCommand(t *testing.T) {
	var output bytes.Buffer
	SettingsListCmd.SetOut(&output)
	t.Cleanup(func() { SettingsListCmd.SetOut(nil) })

	require.NoError(t, SettingsListCmd.RunE(SettingsListCmd, nil))
	require.Contains(t, output.String(), "NetworkProtocol")
	require.Contains(t, output.String(), "ComputerSystem")

	output.Reset()
	require.NoError(t, SettingsListCmd.RunE(SettingsListCmd, []string{"Manager"}))
	require.Contains(t, output.String(), "FirmwareVersion")
	require.ErrorContains(t, SettingsListCmd.RunE(SettingsListCmd, []string{"Unknown"}), "unknown category")
}

func TestSettingsEnvironmentVariablesUseSettingsPrefix(t *testing.T) {
	resetSettingsTestState(t)
	viper.Reset()
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	t.Setenv("SETTINGS_USERNAME", "env-user")
	t.Setenv("SETTINGS_PASSWORD", "env-pass")
	t.Setenv("SETTINGS_INVENTORY_FILE", "/tmp/nodes.yaml")
	t.Setenv("SETTINGS_INPUT_FORMAT", "yaml")
	t.Setenv("SETTINGS_PRESERVE_CONFIG", "PreserveNetwork")

	resolveFlagsFromViper(SettingsGetCmd)
	require.Equal(t, "env-user", username)
	require.Equal(t, "env-pass", password)
	require.Equal(t, "/tmp/nodes.yaml", settingsInventoryFile)
	require.Equal(t, format.FORMAT_YAML, settingsInputFormat)

	resolveFlagsFromViper(SettingsResetCmd)
	require.Equal(t, "PreserveNetwork", settingsPreserveConfig)
}

func TestSettingsFieldRejectsInternalFields(t *testing.T) {
	type resource struct {
		Visible string
		hidden  string
	}
	value := &resource{Visible: "value", hidden: "secret"}
	field, ok := settingsField(value, "Visible")
	require.True(t, ok)
	require.Equal(t, "value", field.String())
	_, ok = settingsField(value, "hidden")
	require.False(t, ok)
}

func TestSettingsConnectReadsJSONAndYAMLInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/redfish/v1/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"@odata.id":"/redfish/v1/","Id":"RootService","Name":"Root Service"}`))
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		name       string
		extension  string
		dataFormat format.DataFormat
		contents   string
	}{
		{
			name:       "JSON",
			extension:  "json",
			dataFormat: format.FORMAT_JSON,
			contents:   fmt.Sprintf(`[{"ID":"x1000c0s0b0","FQDN":%q,"Systems":[{"NodeID":"node-1"}]}]`, server.URL),
		},
		{
			name:       "YAML",
			extension:  "yaml",
			dataFormat: format.FORMAT_YAML,
			contents:   fmt.Sprintf("- ID: x1000c0s0b0\n  FQDN: %s\n  Systems:\n    - NodeID: node-1\n", server.URL),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetSettingsTestState(t)
			path := filepath.Join(t.TempDir(), "nodes."+tt.extension)
			require.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o600))
			settingsInventoryFile = path
			settingsInputFormat = tt.dataFormat
			username, password = "user", "pass"

			client, err := settingsConnect("x1000c0s0b0n0")
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestSettingsConnectRequiresCredentials(t *testing.T) {
	resetSettingsTestState(t)
	_, err := settingsConnect("bmc01")
	require.ErrorContains(t, err, "BMC credentials are required")
}
