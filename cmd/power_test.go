package cmd

import (
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
