package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/models"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPowerIdentifierLookupFromInventory(t *testing.T) {
	inventory := []map[string]any{{
		"ID": "x0c0s0b0", "FQDN": "192.0.2.10",
		"Systems": []models.InventoryDetail{{
			NodeID: "Node0", UUID: "3894755a-8e4c-41d6-a6eb-3c5f4b7d2e10",
			SerialNumber:       "CN75120A3G",
			EthernetInterfaces: []models.EthernetInterface{{MAC: "aa:bb:cc:dd:ee:ff"}, {}},
		}},
	}}
	formats := []struct {
		name       string
		dataFormat format.DataFormat
		marshal    func(any) ([]byte, error)
	}{
		{"json", format.FORMAT_JSON, json.Marshal},
		{"yaml", format.FORMAT_YAML, yaml.Marshal},
	}
	for _, testFormat := range formats {
		t.Run(testFormat.name, func(t *testing.T) {
			contents, err := testFormat.marshal(inventory)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "inventory."+testFormat.name)
			require.NoError(t, os.WriteFile(path, contents, 0o600))
			nodes, err := bmc.ParseInventory(path, testFormat.dataFormat)
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			for _, identifier := range []string{
				"x0c0s0b0n0", "3894755A-8E4C-41D6-A6EB-3C5F4B7D2E10",
				"cn75120a3g", "AA-BB-CC-DD-EE-FF",
			} {
				t.Run(identifier, func(t *testing.T) {
					node, err := findNodeByIdentifier(nodes, identifier)
					require.NoError(t, err)
					require.Equal(t, "Node0", node.NodeID)
					require.Equal(t, "192.0.2.10", node.BmcIP)
				})
			}
			require.Equal(t, []string{"aa:bb:cc:dd:ee:ff"}, nodes[0].MACAddresses)
		})
	}
}

func TestPowerCrawlerConfigPropagatesCACertPath(t *testing.T) {
	store := secrets.NewStaticStore("user", "pass")
	node := bmc.Node{BmcIP: "192.0.2.10"}

	config := powerCrawlerConfig(node, store, false, "/etc/magellan/bmc-ca.pem")
	require.Equal(t, "https://192.0.2.10", config.URI)
	require.Same(t, store, config.CredentialStore)
	require.False(t, config.Insecure)
	require.Equal(t, "/etc/magellan/bmc-ca.pem", config.CACertPath)

	defaultConfig := powerCrawlerConfig(node, store, true, "")
	require.True(t, defaultConfig.Insecure)
	require.Empty(t, defaultConfig.CACertPath)
}

func TestPowerCACertFlagBindsPath(t *testing.T) {
	originalPath := cacertPath
	flag := PowerCmd.Flags().Lookup("cacert")
	originalChanged := flag.Changed
	t.Cleanup(func() {
		cacertPath = originalPath
		flag.Changed = originalChanged
	})

	require.NoError(t, PowerCmd.Flags().Set("cacert", "/etc/magellan/bmc-ca.pem"))
	require.Equal(t, "/etc/magellan/bmc-ca.pem", cacertPath)
	config := powerCrawlerConfig(bmc.Node{BmcIP: "192.0.2.10"}, secrets.NewStaticStore("user", "pass"), false, cacertPath)
	require.Equal(t, cacertPath, config.CACertPath)
}

func TestNormalizeBMCURI(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{address: "192.0.2.10", want: "https://192.0.2.10"},
		{address: "bmc.example.com:8443", want: "https://bmc.example.com:8443"},
		{address: "https://192.0.2.10", want: "https://192.0.2.10"},
		{address: "http://192.0.2.10:8000", want: "http://192.0.2.10:8000"},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			require.Equal(t, test.want, normalizeBMCURI(test.address))
		})
	}
}

// TestPowerCommandExecutes guards against shorthand-flag collisions between the
// power subcommand and the root command's persistent flags. pflag merges the
// root persistent flags into a subcommand's flagset at execution time and
// panics on a duplicate shorthand (e.g. the historical --list-reset-types/-l vs
// --log-level/-l clash). Executing `power --help` forces that merge without
// running the command body, so any reintroduced collision fails here instead of
// at runtime for every user.
func TestPowerCommandExecutes(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"power", "--help"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	// A shorthand collision surfaces as a panic during flag merge; the test
	// fails (rather than the process aborting) if that regresses.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("executing `power --help` returned an error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected help output for `power --help`, got none")
	}
}
