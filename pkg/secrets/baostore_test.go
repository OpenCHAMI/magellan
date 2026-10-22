package secrets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// baoEnvVars are every environment variable the OpenBao SDK reads; they are
// cleared so a developer's real BAO_*/VAULT_* settings never leak into tests.
var baoEnvVars = []string{
	"BAO_ADDR", "BAO_AGENT_ADDR", "BAO_TOKEN", "BAO_NAMESPACE", "BAO_CACERT",
	"BAO_CAPATH", "BAO_CLIENT_CERT", "BAO_CLIENT_KEY", "BAO_SKIP_VERIFY",
	"BAO_TLS_SERVER_NAME", "BAO_CLIENT_TIMEOUT", "BAO_MAX_RETRIES",
	"BAO_ROLE_ID", "BAO_SECRET_ID", "BAO_MOUNT", "BAO_PREFIX", "BAO_LOGIN_PATH",
	"VAULT_ADDR", "VAULT_AGENT_ADDR", "VAULT_TOKEN", "VAULT_NAMESPACE",
	"VAULT_CACERT", "VAULT_CAPATH", "VAULT_CLIENT_CERT", "VAULT_CLIENT_KEY",
	"VAULT_SKIP_VERIFY", "VAULT_TLS_SERVER_NAME", "VAULT_CLIENT_TIMEOUT",
	"VAULT_MAX_RETRIES", "VAULT_ROLE_ID", "VAULT_SECRET_ID", "VAULT_MOUNT",
	"VAULT_PREFIX", "VAULT_LOGIN_PATH",
}

// clearBaoEnv unsets every BAO_*/VAULT_* variable for the duration of the
// test.
func clearBaoEnv(t *testing.T) {
	t.Helper()
	for _, name := range baoEnvVars {
		if val, ok := os.LookupEnv(name); ok {
			require.NoError(t, os.Unsetenv(name))
			t.Cleanup(func() {
				if err := os.Setenv(name, val); err != nil {
					t.Errorf("failed to restore env %s: %v", name, err)
				}
			})
		}
	}
}

// fakeBao is a minimal in-memory OpenBao KV v2 + AppRole server for tests.
type fakeBao struct {
	t *testing.T

	mu      sync.Mutex
	secrets map[string]map[string]any // secret path -> KV data
	seen    []string                  // "METHOD /v1/path" in request order

	token             string // required X-Vault-Token, empty to disable auth
	roleID, secretID  string // accepted AppRole credentials
	clientToken       string // token returned by AppRole login
	requireTokenAfter bool   // reject data/metadata requests until login
	loggedIn          bool
}

func newFakeBao(t *testing.T) *fakeBao {
	t.Helper()
	return &fakeBao{t: t, secrets: make(map[string]map[string]any)}
}

func (f *fakeBao) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func (f *fakeBao) writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	f.t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		f.t.Errorf("failed to encode fake response: %v", err)
	}
}

func (f *fakeBao) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.seen = append(f.seen, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	// AppRole login is the only unauthenticated route.
	if r.URL.Path == "/v1/auth/approle/login" {
		var body struct {
			RoleID   string `json:"role_id"`
			SecretID string `json:"secret_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"errors":["bad login body"]}`, http.StatusBadRequest)
			return
		}
		if body.RoleID != f.roleID || body.SecretID != f.secretID {
			http.Error(w, `{"errors":["invalid credentials"]}`, http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.loggedIn = true
		f.mu.Unlock()
		f.writeJSON(w, http.StatusOK, map[string]any{
			"auth": map[string]any{"client_token": f.clientToken},
		})
		return
	}

	// Enforce auth on every other route.
	if f.token != "" && r.Header.Get("X-Vault-Token") != f.token {
		http.Error(w, `{"errors":["permission denied"]}`, http.StatusForbidden)
		return
	}
	if f.requireTokenAfter && !f.loggedIn {
		http.Error(w, `{"errors":["not logged in"]}`, http.StatusForbidden)
		return
	}

	// Paths look like /v1/<mount>/(data|metadata)/<secret path>; the mount
	// is dynamic so tests can override it.
	rest := strings.TrimPrefix(r.URL.Path, "/v1/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		http.Error(w, `{"errors":["not found"]}`, http.StatusNotFound)
		return
	}
	tail := parts[1]

	switch {
	case tail == "data" || strings.HasPrefix(tail, "data/"):
		path := strings.Trim(strings.TrimPrefix(tail, "data"), "/")
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body struct {
				Data map[string]any `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `{"errors":["bad data body"]}`, http.StatusBadRequest)
				return
			}
			f.mu.Lock()
			f.secrets[path] = body.Data
			f.mu.Unlock()
			f.writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{"metadata": map[string]any{"version": 1}},
			})
		case http.MethodGet:
			f.mu.Lock()
			data, ok := f.secrets[path]
			f.mu.Unlock()
			if !ok {
				http.Error(w, `{"errors":["secret not found"]}`, http.StatusNotFound)
				return
			}
			f.writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{"data": data, "metadata": map[string]any{"version": 1}},
			})
		default:
			http.Error(w, `{"errors":["method not allowed"]}`, http.StatusMethodNotAllowed)
		}

	case tail == "metadata" || strings.HasPrefix(tail, "metadata/"):
		path := strings.Trim(strings.TrimPrefix(tail, "metadata"), "/")
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list") == "true":
			prefix := path
			if prefix != "" {
				prefix += "/"
			}
			keys := []string{}
			f.mu.Lock()
			for p := range f.secrets {
				if strings.HasPrefix(p, prefix) {
					keys = append(keys, strings.TrimPrefix(p, prefix))
				}
			}
			f.mu.Unlock()
			f.writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{"keys": keys},
			})
		case r.Method == http.MethodDelete:
			f.mu.Lock()
			delete(f.secrets, path)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, `{"errors":["method not allowed"]}`, http.StatusMethodNotAllowed)
		}

	default:
		http.Error(w, `{"errors":["not found"]}`, http.StatusNotFound)
	}
}

// newTestBaoStore points the SDK at a fakeBao server via BAO_ADDR/BAO_TOKEN
// and returns the store under test.
func newTestBaoStore(t *testing.T, srv *httptest.Server, token string) *BaoSecretStore {
	t.Helper()
	clearBaoEnv(t)
	t.Setenv("BAO_ADDR", srv.URL)
	if token != "" {
		t.Setenv("BAO_TOKEN", token)
	}
	store, err := NewBaoSecretStore(StoreConfig{Backend: BackendBao})
	require.NoError(t, err)
	return store
}

func TestBaoStoreRoundTrip(t *testing.T) {
	fake := newFakeBao(t)
	fake.token = "test-token"
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store := newTestBaoStore(t, srv, fake.token)

	// A full BMC URI secret ID must be usable as a KV path.
	secretID := "https://172.16.0.10"
	creds := `{"username": "admin", "password": "hunter2"}`

	require.NoError(t, store.StoreSecretByID(secretID, creds))

	got, err := store.GetSecretByID(secretID)
	require.NoError(t, err)
	require.Equal(t, creds, got)

	// The secret ID is stored base64url-encoded as a single path segment.
	encoded := "aHR0cHM6Ly8xNzIuMTYuMC4xMA"
	require.Contains(t, fake.requests(), "PUT /v1/magellan/data/"+encoded)
	require.Contains(t, fake.requests(), "GET /v1/magellan/data/"+encoded)

	// ListSecrets decodes IDs back and fetches every value.
	secretsByID, err := store.ListSecrets()
	require.NoError(t, err)
	require.Equal(t, map[string]string{secretID: creds}, secretsByID)

	// Remove deletes the secret entirely.
	require.NoError(t, store.RemoveSecretByID(secretID))
	require.Contains(t, fake.requests(), "DELETE /v1/magellan/metadata/"+encoded)
	_, err = store.GetSecretByID(secretID)
	require.ErrorContains(t, err, "secret not found")
}

func TestBaoStoreDefaultKeyIsNotEncoded(t *testing.T) {
	fake := newFakeBao(t)
	fake.token = "test-token"
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store := newTestBaoStore(t, srv, fake.token)

	require.NoError(t, store.StoreSecretByID(DEFAULT_KEY, `{"username": "u", "password": "p"}`))
	require.Contains(t, fake.requests(), "PUT /v1/magellan/data/default")

	got, err := store.GetSecretByID(DEFAULT_KEY)
	require.NoError(t, err)
	require.Contains(t, got, `"username": "u"`)

	secretsByID, err := store.ListSecrets()
	require.NoError(t, err)
	require.Contains(t, secretsByID, DEFAULT_KEY)
}

func TestBaoStoreErrors(t *testing.T) {
	t.Run("missing value field", func(t *testing.T) {
		fake := newFakeBao(t)
		fake.token = "test-token"
		fake.secrets["aHR0cHM6Ly8xNzIuMTYuMC4xMw"] = map[string]any{"user": "admin"}
		srv := httptest.NewServer(fake)
		t.Cleanup(srv.Close)
		store := newTestBaoStore(t, srv, fake.token)

		_, err := store.GetSecretByID("https://172.16.0.13")
		require.ErrorContains(t, err, `has no "value" field`)
	})

	t.Run("no credentials in environment", func(t *testing.T) {
		clearBaoEnv(t)
		_, err := NewBaoSecretStore(StoreConfig{Backend: BackendBao})
		require.ErrorContains(t, err, "no credentials for OpenBao/Vault")
	})

	t.Run("rejected by server", func(t *testing.T) {
		fake := newFakeBao(t)
		fake.token = "real-token"
		srv := httptest.NewServer(fake)
		t.Cleanup(srv.Close)
		store := newTestBaoStore(t, srv, "wrong-token")

		_, err := store.GetSecretByID("https://172.16.0.10")
		require.ErrorContains(t, err, "permission denied")
	})
}

func TestBaoStoreAppRoleLogin(t *testing.T) {
	fake := newFakeBao(t)
	fake.roleID = "role-123"
	fake.secretID = "secret-456"
	fake.clientToken = "approle-token"
	fake.requireTokenAfter = true
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	clearBaoEnv(t)
	t.Setenv("BAO_ADDR", srv.URL)
	t.Setenv("BAO_ROLE_ID", fake.roleID)
	t.Setenv("BAO_SECRET_ID", fake.secretID)

	store, err := NewBaoSecretStore(StoreConfig{Backend: BackendBao})
	require.NoError(t, err)
	require.Contains(t, fake.requests(), "PUT /v1/auth/approle/login")

	// The token from login must be used for subsequent KV requests.
	require.NoError(t, store.StoreSecretByID("https://172.16.0.10", `{"username": "u", "password": "p"}`))
}

func TestBaoStoreListEmptyAndSubdirectories(t *testing.T) {
	fake := newFakeBao(t)
	fake.token = "test-token"
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store := newTestBaoStore(t, srv, fake.token)

	// Empty store lists as empty, not an error.
	secretsByID, err := store.ListSecrets()
	require.NoError(t, err)
	require.Empty(t, secretsByID)

	// Subdirectories (keys ending in "/") are skipped, not listed.
	fake.mu.Lock()
	fake.secrets["sub/"] = nil
	fake.secrets["aHR0cHM6Ly8xNzIuMTYuMC4xNA"] = map[string]any{"value": `{"username": "u", "password": "p"}`}
	fake.mu.Unlock()

	secretsByID, err = store.ListSecrets()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"https://172.16.0.14": `{"username": "u", "password": "p"}`}, secretsByID)
}

func TestBaoStoreRespectsBAOEnvOverrides(t *testing.T) {
	fake := newFakeBao(t)
	fake.token = "test-token"
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	clearBaoEnv(t)
	t.Setenv("BAO_ADDR", srv.URL)
	t.Setenv("BAO_TOKEN", fake.token)
	t.Setenv("BAO_PREFIX", "clusters/x1")

	store, err := NewBaoSecretStore(StoreConfig{Backend: BackendBao})
	require.NoError(t, err)

	require.NoError(t, store.StoreSecretByID(DEFAULT_KEY, `{"username": "u", "password": "p"}`))
	require.Contains(t, fake.requests(), "PUT /v1/magellan/data/clusters/x1/default")
}

func TestBaoStoreMountOverride(t *testing.T) {
	fake := newFakeBao(t)
	fake.token = "test-token"
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	clearBaoEnv(t)
	t.Setenv("BAO_ADDR", srv.URL)
	t.Setenv("BAO_TOKEN", fake.token)

	store, err := NewBaoSecretStore(StoreConfig{Backend: BackendVault, Mount: "custom"})
	require.NoError(t, err)
	require.NoError(t, store.StoreSecretByID(DEFAULT_KEY, `{"username": "u", "password": "p"}`))
	require.Contains(t, fake.requests(), "PUT /v1/custom/data/default")
}

func TestNewStoreBackends(t *testing.T) {
	t.Run("empty backend means local", func(t *testing.T) {
		clearBaoEnv(t)
		t.Setenv("MASTER_KEY", mustMasterKey(t))
		store, err := NewStore(StoreConfig{LocalFile: t.TempDir() + "/secrets.json"})
		require.NoError(t, err)
		require.IsType(t, &LocalSecretStore{}, store)
	})

	t.Run("local backend round trip", func(t *testing.T) {
		clearBaoEnv(t)
		t.Setenv("MASTER_KEY", mustMasterKey(t))
		store, err := NewStore(StoreConfig{Backend: BackendLocal, LocalFile: t.TempDir() + "/secrets.json"})
		require.NoError(t, err)
		require.IsType(t, &LocalSecretStore{}, store)
	})

	t.Run("bao backend via NewStore", func(t *testing.T) {
		fake := newFakeBao(t)
		fake.token = "test-token"
		srv := httptest.NewServer(fake)
		t.Cleanup(srv.Close)
		store := newTestBaoStore(t, srv, fake.token)
		require.IsType(t, &BaoSecretStore{}, store)
	})

	t.Run("unknown backend", func(t *testing.T) {
		_, err := NewStore(StoreConfig{Backend: Backend("consul")})
		require.ErrorContains(t, err, "unknown secrets backend")
	})
}

func TestBackendFlagValue(t *testing.T) {
	var b Backend
	require.Equal(t, "Backend", b.Type())
	for _, name := range []string{"local", "bao", "vault"} {
		require.NoError(t, b.Set(name))
		require.Equal(t, Backend(name), b)
		require.True(t, b.Valid())
		require.Contains(t, BackendHelpMap, b.String())
	}
	require.ErrorContains(t, b.Set("consul"), "invalid secrets backend")
	require.False(t, Backend("consul").Valid())
}

// mustMasterKey returns a valid master key for the local store.
func mustMasterKey(t *testing.T) string {
	t.Helper()
	key, err := GenerateMasterKey()
	require.NoError(t, err)
	return key
}
