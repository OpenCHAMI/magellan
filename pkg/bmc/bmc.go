package bmc

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
)

type BMCCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetBMCCredentialsDefault(store secrets.SecretStore) (BMCCredentials, error) {
	var creds BMCCredentials
	if strCreds, err := store.GetSecretByID(secrets.DEFAULT_KEY); err != nil {
		return creds, fmt.Errorf("get default BMC credentials from secret store: %w", err)
	} else {
		// Default URI credentials found, use them.
		if err = json.Unmarshal([]byte(strCreds), &creds); err != nil {
			return creds, fmt.Errorf("get default BMC credentials from secret store: failed to unmarshal: %w", err)
		}
		return creds, nil
	}
}

func GetBMCCredentials(store secrets.SecretStore, id string) (BMCCredentials, error) {
	var creds BMCCredentials
	if strCreds, err := store.GetSecretByID(id); err != nil {
		return creds, fmt.Errorf("get BMC credentials from secret store: %w", err)
	} else {
		// Specific URI credentials found, use them.
		if err = json.Unmarshal([]byte(strCreds), &creds); err != nil {
			return creds, fmt.Errorf("get BMC credentials from secret store: failed to unmarshal: %w", err)
		}
	}

	return creds, nil
}

func GetBMCCredentialsOrDefault(store secrets.SecretStore, id string) BMCCredentials {
	var (
		creds BMCCredentials
		err   error
	)

	if id == "" {
		return creds
	}

	if id == secrets.DEFAULT_KEY {
		creds, _ = GetBMCCredentialsDefault(store)
		return creds
	}

	if creds, err = GetBMCCredentials(store, id); err != nil {
		if defaultSecret, err := GetBMCCredentialsDefault(store); err == nil {
			// Default credentials found, use them.
			creds = defaultSecret
		}
	}

	return creds
}
