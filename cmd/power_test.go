package cmd

import (
	"bytes"
	"testing"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

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
