package bmc

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestGetUserPass(t *testing.T) {
	t.Run("nil store errors", func(t *testing.T) {
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: nil}
		_, err := cfg.GetUserPass()
		if err == nil || !strings.Contains(err.Error(), "credential store is invalid") {
			t.Fatalf("GetUserPass with nil store err = %v, want 'credential store is invalid'", err)
		}
	})

	t.Run("blank credentials error", func(t *testing.T) {
		// A store that resolves to empty username/password must be reported as an
		// error rather than silently producing an unauthenticated connection.
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: secrets.NewStaticStore("", "")}
		_, err := cfg.GetUserPass()
		if err == nil || !strings.Contains(err.Error(), "credentials blank") {
			t.Fatalf("GetUserPass with blank creds err = %v, want 'credentials blank'", err)
		}
	})

	t.Run("valid credentials returned", func(t *testing.T) {
		cfg := ConnConfig{URI: "https://bmc.example", CredentialStore: secrets.NewStaticStore("alice", "s3cret")}
		creds, err := cfg.GetUserPass()
		if err != nil {
			t.Fatalf("GetUserPass unexpected error: %v", err)
		}
		if creds.Username != "alice" || creds.Password != "s3cret" {
			t.Fatalf("GetUserPass creds = %+v, want {alice s3cret}", creds)
		}
	})
}
