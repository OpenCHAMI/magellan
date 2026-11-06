package cmd

import (
	"fmt"
	"testing"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

// Only GetSecretByID is implemented: any attempt to write through this store
// panics, guarding against persistence of invocation-local overrides.
type readOnlyCredentials struct{ secrets.SecretStore }

func (readOnlyCredentials) GetSecretByID(id string) (string, error) {
	return `{"username":"stored-user","password":"stored-password"}`, nil
}

type fallbackCredentials struct{ secrets.SecretStore }

func (fallbackCredentials) GetSecretByID(id string) (string, error) {
	if id == secrets.DEFAULT_KEY {
		return `{"username":"default","password":"password"}`, nil
	}
	if id == "malformed" {
		return "not json", nil
	}
	return "", fmt.Errorf("missing credential")
}

func TestCredentialOverridesPreserveFallbackAndErrors(t *testing.T) {
	s := credentialOverrides{fallbackCredentials{}, "override", ""}
	_, err := s.GetSecretByID("missing")
	require.Error(t, err)
	_, err = s.GetSecretByID("malformed")
	require.Error(t, err)
	creds := bmc.GetBMCCredentialsOrDefault(s, "missing")
	require.Equal(t, "override", creds.Username)
	require.Equal(t, "password", creds.Password)
}

func TestCredentialOverrides(t *testing.T) {
	for _, tc := range []struct{ user, password, wantUser, wantPassword string }{
		{"override", "", "override", "stored-password"},
		{"", "override", "stored-user", "override"},
		{"", "", "stored-user", "stored-password"},
	} {
		s := credentialOverrides{readOnlyCredentials{}, tc.user, tc.password}
		for _, id := range []string{"https://bmc", secrets.DEFAULT_KEY} {
			creds, err := bmc.GetBMCCredentials(s, id)
			require.NoError(t, err)
			require.Equal(t, tc.wantUser, creds.Username)
			require.Equal(t, tc.wantPassword, creds.Password)
		}
	}
}

func TestSecretsTLSFlags(t *testing.T) {
	for _, cmd := range secretsCmd.Commands() {
		require.NotNil(t, cmd.InheritedFlags().Lookup("cacert"), cmd.Name())
		f := cmd.InheritedFlags().Lookup("insecure")
		require.NotNil(t, f, cmd.Name())
		require.Empty(t, f.Shorthand)
		require.Equal(t, "bmc", f.NoOptDefVal)
	}
	require.Equal(t, "i", secretsStoreCmd.Flags().Lookup("input-file").Shorthand)
}
