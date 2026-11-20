package bmc

import (
	"errors"
	"testing"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

type errorStore struct{}

func (errorStore) GetSecretByID(string) (string, error)    { return "", errors.New("missing") }
func (errorStore) StoreSecretByID(string, string) error    { return nil }
func (errorStore) ListSecrets() (map[string]string, error) { return nil, nil }
func (errorStore) RemoveSecretByID(string) error           { return nil }

type mapStore map[string]string

func (s mapStore) GetSecretByID(id string) (string, error) {
	v, ok := s[id]
	if !ok {
		return "", errors.New("missing")
	}
	return v, nil
}
func (mapStore) StoreSecretByID(string, string) error    { return nil }
func (mapStore) ListSecrets() (map[string]string, error) { return nil, nil }
func (mapStore) RemoveSecretByID(string) error           { return nil }

func TestCredentials(t *testing.T) {
	store := mapStore{
		"node":              `{"username":"node-user","password":"node-pass"}`,
		secrets.DEFAULT_KEY: `{"username":"default-user","password":"default-pass"}`,
	}
	got, err := GetBMCCredentials(store, "node")
	require.NoError(t, err)
	require.Equal(t, "node-user", got.Username)
	got, err = GetBMCCredentialsDefault(store)
	require.NoError(t, err)
	require.Equal(t, "default-user", got.Username)
	require.Equal(t, got, GetBMCCredentialsOrDefault(store, "missing"))
	require.Equal(t, BMCCredentials{}, GetBMCCredentialsOrDefault(store, ""))
	require.Error(t, func() error { _, err := GetBMCCredentials(nil, "node"); return err }())
	require.Error(t, func() error { _, err := GetBMCCredentialsDefault(errorStore{}); return err }())
	_, err = GetBMCCredentials(mapStore{"node": "{"}, "node")
	require.Error(t, err)
}

func TestCredentialsDefaultPortEquivalence(t *testing.T) {
	const node = `{"username":"node-user","password":"node-pass"}`
	const fallback = `{"username":"fallback","password":"fallback"}`
	for _, tc := range []struct {
		name, requested, stored string
	}{
		{"https omitted", "https://172.16.0.101", "https://172.16.0.101:443"},
		{"https explicit", "https://bmc.example:443", "https://bmc.example"},
		{"http omitted", "http://bmc.example", "http://bmc.example:80"},
		{"http explicit", "http://bmc.example:80", "http://bmc.example"},
		{"IPv6 omitted", "https://[2001:db8::1]", "https://[2001:db8::1]:443"},
		{"IPv6 explicit", "https://[2001:db8::1]:443", "https://[2001:db8::1]"},
		{"path preserved", "https://bmc.example/redfish", "https://bmc.example:443/redfish"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mapStore{tc.stored: node, secrets.DEFAULT_KEY: fallback}
			got, err := GetBMCCredentials(store, tc.requested)
			require.NoError(t, err)
			require.Equal(t, "node-user", got.Username)
			require.Equal(t, got, GetBMCCredentialsOrDefault(store, tc.requested))

			// Exact IDs take precedence even if both forms are present.
			store[tc.requested] = fallback
			got, err = GetBMCCredentials(store, tc.requested)
			require.NoError(t, err)
			require.Equal(t, "fallback", got.Username)

			// An invalid exact value is not silently replaced by an alias.
			store[tc.requested] = "{"
			_, err = GetBMCCredentials(store, tc.requested)
			require.ErrorContains(t, err, "unmarshal")
		})
	}
}

func TestCredentialsDoNotAliasDistinctEndpoints(t *testing.T) {
	for _, id := range []string{
		"https://bmc.example:8443", "https://bmc.example:80",
		"http://bmc.example:443", "ssh://bmc.example", "bmc.example",
		"https://user@bmc.example", "https://bmc.example:invalid",
	} {
		t.Run(id, func(t *testing.T) {
			require.Empty(t, defaultPortCredentialID(id))
			store := mapStore{
				"https://bmc.example": `{"username":"wrong-endpoint"}`,
				secrets.DEFAULT_KEY:   `{"username":"default"}`,
			}
			_, err := GetBMCCredentials(store, id)
			require.Error(t, err)
			require.Equal(t, "default", GetBMCCredentialsOrDefault(store, id).Username)
		})
	}
}
