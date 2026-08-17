package bmc

import (
	"errors"
	"testing"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
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
