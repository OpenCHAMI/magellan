package secrets

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticStore(t *testing.T) {
	store := NewStaticStore("user", "pass")
	secret, err := store.GetSecretByID("anything")
	require.NoError(t, err)
	require.JSONEq(t, `{"username":"user","password":"pass"}`, secret)
	require.NoError(t, store.StoreSecretByID("id", "value"))
	listed, err := store.ListSecrets()
	require.NoError(t, err)
	require.Contains(t, listed["static_creds"], "user")
	require.NoError(t, store.RemoveSecretByID("id"))
}

func TestLocalStoreErrorsRemovalAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	key, err := GenerateMasterKey()
	require.NoError(t, err)
	_, err = NewLocalSecretStore("not-hex", path, true)
	require.Error(t, err)
	_, err = NewLocalSecretStore(key, path, false)
	require.Error(t, err)

	store, err := NewLocalSecretStore(key, path, true)
	require.NoError(t, err)
	_, err = NewLocalSecretStore(key, path, false)
	require.NoError(t, err)
	require.Error(t, store.RemoveSecretByID("missing"))
	require.NoError(t, store.StoreSecretByID("id", "secret"))
	require.NoError(t, store.RemoveSecretByID("id"))
	_, err = store.GetSecretByID("id")
	require.Error(t, err)

	reloaded, err := NewLocalSecretStore(key, path, false)
	require.NoError(t, err)
	_, err = reloaded.GetSecretByID("id")
	require.Error(t, err)

	listed, err := reloaded.ListSecrets()
	require.NoError(t, err)
	listed["external"] = "mutation"
	require.NotContains(t, reloaded.Secrets, "external")
}

func TestOpenStore(t *testing.T) {
	_, err := OpenStore("")
	require.Error(t, err)
	t.Setenv("MASTER_KEY", "")
	_, err = OpenStore(filepath.Join(t.TempDir(), "secrets.json"))
	require.Error(t, err)
	key, err := GenerateMasterKey()
	require.NoError(t, err)
	t.Setenv("MASTER_KEY", key)
	_, err = OpenStore(filepath.Join(t.TempDir(), "secrets.json"))
	require.NoError(t, err)
}
