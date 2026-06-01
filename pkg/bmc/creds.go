package bmc

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetCredentialsDefault(store secrets.SecretStore) (Credentials, error) {
	var creds Credentials
	if store == nil {
		return creds, fmt.Errorf("invalid secrets store")
	}
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

func GetCredentials(store secrets.SecretStore, id string) (Credentials, error) {
	var creds Credentials
	if store == nil {
		return creds, fmt.Errorf("invalid secrets store")
	}
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

func GetCredentialsOrDefault(store secrets.SecretStore, id string) Credentials {
	var (
		creds Credentials
		err   error
	)

	if id == "" {
		return creds
	}

	if id == secrets.DEFAULT_KEY {
		creds, _ = GetCredentialsDefault(store)
		return creds
	}

	if creds, err = GetCredentials(store, id); err != nil {
		if defaultSecret, err := GetCredentialsDefault(store); err == nil {
			// Default credentials found, use them.
			creds = defaultSecret
		}
	}

	return creds
}

func LoadCredsWithConfig(config Config) (Credentials, error) {
	// NOTE: it is possible for the SecretStore to be nil, so we need a check
	if config.CredentialStore == nil {
		return Credentials{}, fmt.Errorf("credential store is invalid")
	}
	if creds := loadCreds(config.CredentialStore, config.URI); creds == (Credentials{}) {
		return creds, fmt.Errorf("%s: credentials blank for BMC", config.URI)
	} else {
		return creds, nil
	}

}

func loadCreds(store secrets.SecretStore, id string) Credentials {
	var (
		creds Credentials
		err   error
	)

	if id == "" {
		log.Error().Msg("failed to get BMC credentials: id was empty")
		return creds
	}

	if id == secrets.DEFAULT_KEY {
		log.Info().Msg("fetching default credentials")
		if creds, err = GetCredentialsDefault(store); err != nil {
			log.Warn().Err(err).Msg("failed to get default credentials")
		} else {
			log.Info().Msg("default credentials found, using")
		}
		return creds
	}

	if creds, err = GetCredentials(store, id); err != nil {
		// Specific credentials for URI not found, fetch default.
		log.Warn().Str("id", id).Msg("specific credentials not found, falling back to default")
		if defaultSecret, err := GetCredentialsDefault(store); err != nil {
			// We've exhausted all options, the credentials will be blank unless
			// overridden by a CLI flag.
			log.Warn().Str("id", id).Err(err).Msg("no default credentials were set, they will be blank unless overridden by CLI flags")
		} else {
			// Default credentials found, use them.
			log.Info().Str("id", id).Msg("default credentials found, using")
			creds = defaultSecret
		}
	} else {
		log.Info().Str("id", id).Msg("specific credentials found, using")
	}

	return creds
}
