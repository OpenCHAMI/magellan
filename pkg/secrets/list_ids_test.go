package secrets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type legacyOnlyStore struct{ SecretStore }

func TestIDsListingDoesNotFallBackToValues(t *testing.T) {
	// The embedded nil store would panic if ListSecrets were called.
	_, err := ListSecretIDs(legacyOnlyStore{})
	require.ErrorContains(t, err, "IDs-only listing")
}

func TestBaoListIDsWithoutReadPermission(t *testing.T) {
	clearBaoEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isList := r.Method == "LIST" || (r.Method == "GET" && r.URL.Query().Get("list") == "true")
		if !isList || r.URL.Path != "/v1/magellan/metadata/team" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprint(w, `{"data":{"keys":["eg","default","YQ","subdir/"]}}`)
	}))
	defer server.Close()
	t.Setenv("BAO_ADDR", server.URL)
	t.Setenv("BAO_TOKEN", "list-only")
	store, err := NewBaoSecretStore(StoreConfig{Prefix: "team"})
	require.NoError(t, err)
	defer store.Close()
	ids, err := ListSecretIDs(store)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "default", "z"}, ids)
}

func TestBuiltInListIDs(t *testing.T) {
	ids, err := ListSecretIDs(&LocalSecretStore{Secrets: map[string]string{"z": "secret", "a": "secret"}})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "z"}, ids)
	ids, err = ListSecretIDs(NewStaticStore("user", "password"))
	require.NoError(t, err)
	require.Equal(t, []string{"static_creds"}, ids)
	ids, err = ListSecretIDs(&LocalSecretStore{})
	require.NoError(t, err)
	require.Equal(t, []string{}, ids)
}
