package secrets

import (
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBaoEnvironmentCAPrecedesFallback(t *testing.T) {
	for _, env := range []string{"BAO_CACERT", "VAULT_CACERT", "BAO_CACERT_BYTES", "VAULT_CACERT_BYTES", "BAO_CAPATH", "VAULT_CAPATH"} {
		t.Run(env, func(t *testing.T) {
			clearBaoEnv(t)
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"data":{"data":{"value":"credentials"}}}`)
			}))
			defer srv.Close()
			cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
			dir := t.TempDir()
			path := filepath.Join(dir, "ca.pem")
			require.NoError(t, os.WriteFile(path, cert, 0600))
			value := path
			switch env {
			case "BAO_CACERT_BYTES", "VAULT_CACERT_BYTES":
				value = string(cert)
			case "BAO_CAPATH", "VAULT_CAPATH":
				value = dir
			}
			t.Setenv(env, value)
			t.Setenv("BAO_ADDR", srv.URL)
			t.Setenv("BAO_TOKEN", "test")
			store, err := NewBaoSecretStore(StoreConfig{CACert: filepath.Join(dir, "missing.pem")})
			require.NoError(t, err)
			defer store.Close()
			_, err = store.GetSecretByID("default")
			require.NoError(t, err)
		})
	}
}

func TestBaoAppRoleReauthAndClose(t *testing.T) {
	clearBaoEnv(t)
	var logins atomic.Int32
	var issued atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/approle/login" {
			n := logins.Add(1)
			// Force one transient login failure after the initial token.
			if n == 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			issued.Store(n)
			fmt.Fprintf(w, `{"auth":{"client_token":"token-%d","lease_duration":1,"renewable":false}}`, n)
			return
		}
		if r.Header.Get("X-Vault-Token") != fmt.Sprintf("token-%d", issued.Load()) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprint(w, `{"data":{"data":{"value":"credentials"}}}`)
	}))
	defer server.Close()
	t.Setenv("BAO_ADDR", server.URL)
	t.Setenv("BAO_ROLE_ID", "role")
	t.Setenv("BAO_SECRET_ID", "secret")
	t.Setenv("BAO_MAX_RETRIES", "0")
	store, err := NewBaoSecretStore(StoreConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.Eventually(t, func() bool { return logins.Load() >= 3 }, 10*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		_, err := store.GetSecretByID("default")
		return err == nil
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, store.Close())
	require.NoError(t, store.Close())
	select {
	case <-store.done:
	default:
		t.Fatal("Close must wait for renewal worker to stop")
	}
}

func TestBaoCAFallback(t *testing.T) {
	clearBaoEnv(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"data":{"value":"credentials"}}}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "ca.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(path, cert, 0600))
	t.Setenv("BAO_ADDR", server.URL)
	t.Setenv("BAO_TOKEN", "test")
	store, err := NewBaoSecretStore(StoreConfig{CACert: path})
	require.NoError(t, err)
	defer store.Close()
	_, err = store.GetSecretByID("default")
	require.NoError(t, err)
}

func TestBaoAppRoleRenewsToken(t *testing.T) {
	clearBaoEnv(t)
	var renewals atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/token/renew-self" {
			renewals.Add(1)
		}
		fmt.Fprint(w, `{"auth":{"client_token":"token","lease_duration":60,"renewable":true}}`)
	}))
	defer server.Close()
	t.Setenv("BAO_ADDR", server.URL)
	t.Setenv("BAO_ROLE_ID", "role")
	t.Setenv("BAO_SECRET_ID", "secret")
	store, err := NewBaoSecretStore(StoreConfig{})
	require.NoError(t, err)
	defer store.Close()
	require.Eventually(t, func() bool { return renewals.Load() > 0 }, 5*time.Second, 10*time.Millisecond)
}
