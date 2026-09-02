package crawler

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

func TestGetBMCClientWithCACertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/redfish/v1/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"@odata.id":"/redfish/v1/","Id":"RootService","Name":"Root Service"}`))
	}))
	t.Cleanup(server.Close)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(caPath, caPEM, 0o600))

	client, err := GetBMCClient(CrawlerConfig{
		URI:             server.URL,
		CACertPath:      caPath,
		CredentialStore: secrets.NewStaticStore("user", "pass"),
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestGetBMCClientRejectsInvalidCACertificate(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("not a certificate"), 0o600))

	_, err := GetBMCClient(CrawlerConfig{
		URI:             "https://bmc.invalid",
		CACertPath:      caPath,
		CredentialStore: secrets.NewStaticStore("user", "pass"),
	})
	require.ErrorContains(t, err, "failed to parse CA certificate")
}
