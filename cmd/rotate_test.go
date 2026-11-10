package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/openchami/magellan/internal/insecure"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

func TestReadLocalRotationPasswordIndependentOfSelectedBackend(t *testing.T) {
	masterKey, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", masterKey)

	path := filepath.Join(t.TempDir(), "rotation-secrets.json")
	store, err := secrets.NewLocalSecretStore(masterKey, path, true)
	require.NoError(t, err)
	credential, err := json.Marshal(bmc.BMCCredentials{
		Username: "test_user",
		Password: "replacement-password",
	})
	require.NoError(t, err)
	require.NoError(t, store.StoreSecretByID("rotation", string(credential)))

	oldBackend := secretsBackend
	secretsBackend = secrets.BackendBao
	t.Cleanup(func() { secretsBackend = oldBackend })

	got, err := readLocalRotationPassword(path, "rotation")
	require.NoError(t, err)
	require.Equal(t, "replacement-password", got)
}

func TestReadLocalRotationPasswordErrors(t *testing.T) {
	masterKey, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", masterKey)

	path := filepath.Join(t.TempDir(), "rotation-secrets.json")
	store, err := secrets.NewLocalSecretStore(masterKey, path, true)
	require.NoError(t, err)
	require.NoError(t, store.StoreSecretByID("malformed", "not-json"))
	require.NoError(t, store.StoreSecretByID("empty", `{"username":"test_user","password":""}`))

	_, err = readLocalRotationPassword(path, "missing")
	require.ErrorContains(t, err, `read replacement credential "missing"`)
	_, err = readLocalRotationPassword(path, "malformed")
	require.ErrorContains(t, err, "not valid credential JSON")
	_, err = readLocalRotationPassword(path, "empty")
	require.ErrorContains(t, err, "empty password")
	_, err = readLocalRotationPassword("", "rotation")
	require.ErrorContains(t, err, "required")
}

func TestResolveRotationPasswordUsesEnvironmentBeforeLocalSecret(t *testing.T) {
	masterKey, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", masterKey)
	t.Setenv(rotationPasswordEnv, "environment-password")

	path := filepath.Join(t.TempDir(), "rotation-secrets.json")
	store, err := secrets.NewLocalSecretStore(masterKey, path, true)
	require.NoError(t, err)
	credential, err := json.Marshal(bmc.BMCCredentials{Username: "test_user", Password: "stored-password"})
	require.NoError(t, err)
	require.NoError(t, store.StoreSecretByID("rotation", string(credential)))

	got, err := resolveRotationPassword("rotation", path)
	require.NoError(t, err)
	require.Equal(t, "environment-password", got)
}

func TestResolveRotationPasswordFallsBackToLocalSecret(t *testing.T) {
	masterKey, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", masterKey)
	t.Setenv(rotationPasswordEnv, "")

	path := filepath.Join(t.TempDir(), "rotation-secrets.json")
	store, err := secrets.NewLocalSecretStore(masterKey, path, true)
	require.NoError(t, err)
	credential, err := json.Marshal(bmc.BMCCredentials{Username: "test_user", Password: "stored-password"})
	require.NoError(t, err)
	require.NoError(t, store.StoreSecretByID("rotation", string(credential)))

	got, err := resolveRotationPassword("rotation", path)
	require.NoError(t, err)
	require.Equal(t, "stored-password", got)
}

func TestResolveRotationPasswordRequiresInput(t *testing.T) {
	t.Setenv(rotationPasswordEnv, "")
	_, err := resolveRotationPassword("", "")
	require.ErrorContains(t, err, rotationPasswordEnv)
}

func TestRotateAccountCommandUsesSecretIDInsteadOfPasswordFile(t *testing.T) {
	require.NotNil(t, rotateAccountCmd.Flags().Lookup("new-password-secret-id"))
	require.NotNil(t, rotateAccountCmd.Flags().Lookup("new-password-secrets-file"))
	require.NotNil(t, rotateAccountCmd.Flags().Lookup("secrets-file"))
	require.Nil(t, rotateAccountCmd.Flags().Lookup("new-password-file"))
	require.Contains(t, rotateAccountCmd.Long, "selected --secrets-backend")
	require.Contains(t, rotateAccountCmd.Long, rotationPasswordEnv)
}

func TestRecoverRotationCommandRegistered(t *testing.T) {
	recover, _, err := SettingsCmd.Find([]string{"recover-rotation"})
	require.NoError(t, err)
	require.Same(t, recoverAccountRotationCmd, recover)
	require.NotNil(t, recoverAccountRotationCmd.Flags().Lookup("secrets-file"))
	require.NotNil(t, recoverAccountRotationCmd.Flags().Lookup("insecure"))
	require.Nil(t, recoverAccountRotationCmd.Flags().Lookup("password"))
	require.Contains(t, recoverAccountRotationCmd.Long, "If neither password works")
}

type rotateCommandFixture struct {
	mu       sync.Mutex
	username string
	password string
}

func (f *rotateCommandFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/redfish/v1/" {
		username, password, ok := r.BasicAuth()
		f.mu.Lock()
		valid := ok && username == f.username && password == f.password
		f.mu.Unlock()
		if !valid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/","AccountService":{"@odata.id":"/redfish/v1/AccountService"}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/AccountService":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/AccountService","Accounts":{"@odata.id":"/redfish/v1/AccountService/Accounts"}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/AccountService/Accounts":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/AccountService/Accounts","Members":[{"@odata.id":"/redfish/v1/AccountService/Accounts/2"}]}`)
	case r.URL.Path == "/redfish/v1/AccountService/Accounts/2" && r.Method == http.MethodGet:
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/AccountService/Accounts/2","Id":"2","UserName":"test_user"}`)
	case r.URL.Path == "/redfish/v1/AccountService/Accounts/2" && r.Method == http.MethodPatch:
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad patch", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.password = payload["Password"]
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func TestRotateAccountCommandUsesLocalReplacementAndSelectedDestinationStore(t *testing.T) {
	oldBackend := secretsBackend
	oldSourceFile, oldSourceID := rotationSecretsFile, rotationPasswordSecretID
	oldDestinationFile := bmc.SettingsSecretsFile
	oldCACert, oldScope := cacertPath, insecureScope
	oldInsecure, oldInsecureTLS, oldSettingsBackend := bmc.SettingsInsecure, bmc.SettingsInsecureTLS, bmc.SettingsSecretsBackend
	t.Cleanup(func() {
		secretsBackend = oldBackend
		rotationSecretsFile, rotationPasswordSecretID = oldSourceFile, oldSourceID
		bmc.SettingsSecretsFile = oldDestinationFile
		cacertPath, insecureScope = oldCACert, oldScope
		bmc.SettingsInsecure, bmc.SettingsInsecureTLS = oldInsecure, oldInsecureTLS
		bmc.SettingsSecretsBackend = oldSettingsBackend
	})

	masterKey, err := secrets.GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", masterKey)

	sourcePath := filepath.Join(t.TempDir(), "rotation-secrets.json")
	source, err := secrets.NewLocalSecretStore(masterKey, sourcePath, true)
	require.NoError(t, err)
	replacementJSON, err := json.Marshal(bmc.BMCCredentials{Username: "test_user", Password: "replacement"})
	require.NoError(t, err)
	require.NoError(t, source.StoreSecretByID("next", string(replacementJSON)))

	destinationPath := filepath.Join(t.TempDir(), "bmc-secrets.json")
	destination, err := secrets.NewLocalSecretStore(masterKey, destinationPath, true)
	require.NoError(t, err)

	fixture := &rotateCommandFixture{username: "test_user", password: "current"}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	currentJSON, err := json.Marshal(bmc.BMCCredentials{Username: "test_user", Password: "current"})
	require.NoError(t, err)
	require.NoError(t, destination.StoreSecretByID(server.URL, string(currentJSON)))

	secretsBackend = secrets.BackendLocal
	rotationSecretsFile = sourcePath
	rotationPasswordSecretID = "next"
	bmc.SettingsSecretsFile = destinationPath
	bmc.SettingsInsecure, bmc.SettingsInsecureTLS = false, false
	bmc.SettingsSecretsBackend = string(secrets.BackendLocal)
	cacertPath = ""
	insecureScope = insecure.None

	var output bytes.Buffer
	rotateAccountCmd.SetOut(&output)
	t.Cleanup(func() { rotateAccountCmd.SetOut(nil) })
	require.NoError(t, rotateAccountCmd.RunE(rotateAccountCmd, []string{server.URL, "2"}))
	require.Contains(t, output.String(), "saved to the selected secrets backend")
	require.Equal(t, "replacement", fixture.password)
	reloadedDestination, err := secrets.NewLocalSecretStore(masterKey, destinationPath, false)
	require.NoError(t, err)
	updatedJSON, err := reloadedDestination.GetSecretByID(server.URL)
	require.NoError(t, err)
	var updated bmc.BMCCredentials
	require.NoError(t, json.Unmarshal([]byte(updatedJSON), &updated))
	require.Equal(t, bmc.BMCCredentials{Username: "test_user", Password: "replacement"}, updated)
}
