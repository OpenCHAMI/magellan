package bmc

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

func TestConnectWithCredentialsWithCACertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/redfish/v1/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"@odata.id":"/redfish/v1/","Id":"RootService","Name":"Root Service"}`))
	}))
	t.Cleanup(server.Close)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(caPath, caPEM, 0o600))

	client, err := ConnectWithCredentials(server.URL, secrets.NewStaticStore("user", "pass"), false, caPath)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestConnectWithCredentialsRejectsInvalidCACertificate(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("not a certificate"), 0o600))

	_, err := ConnectWithCredentials("https://bmc.invalid", secrets.NewStaticStore("user", "pass"), false, caPath)
	require.ErrorContains(t, err, "failed to parse CA certificate")
}