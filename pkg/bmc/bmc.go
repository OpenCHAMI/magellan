package bmc

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
)

type BMCCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetBMCCredentials(store secrets.SecretStore, id string) (BMCCredentials, error) {
	var creds BMCCredentials
	if id == secrets.DEFAULT_KEY {
		log.Info().Msg("fetching default credentials")
		if uriCreds, err := store.GetSecretByID(id); err != nil {
			log.Warn().Err(err).Msg("failed to get default credentials")
			return creds, fmt.Errorf("get default credentials: %w", err)
		} else {
			if err := json.Unmarshal([]byte(uriCreds), &creds); err != nil {
				log.Error().Err(err).Msg("failed to unmarshal default credentials")
				return creds, fmt.Errorf("unmarshal default credentials: %w", err)
			} else {
				log.Info().Msg("default credentials found, using")
			}
		}

		return creds, nil
	}

	if uriCreds, err := store.GetSecretByID(id); err != nil {
		// Specific credentials for URI not found, fetch default.
		log.Warn().Str("id", id).Msg("specific credentials not found, falling back to default")
		defaultSecret, err := store.GetSecretByID(secrets.DEFAULT_KEY)
		if err != nil {
			// We've exhausted all options, the credentials will be blank unless
			// overridden by a CLI flag.
			log.Warn().Str("id", id).Err(err).Msg("no default credentials were set, they will be blank unless overridden by CLI flags")
		} else {
			// Default credentials found, use them.
			if err = json.Unmarshal([]byte(defaultSecret), &creds); err != nil {
				log.Warn().Str("id", id).Err(err).Msg("failed to unmarshal default secrets store credentials")
				return creds, err
			} else {
				log.Info().Str("id", id).Msg("default credentials found, using")
			}
		}
	} else {
		// Specific URI credentials found, use them.
		if err = json.Unmarshal([]byte(uriCreds), &creds); err != nil {
			log.Warn().Str("id", id).Err(err).Msg("failed to unmarshal specific credentials")
			return creds, err
		} else {
			log.Info().Str("id", id).Msg("specific credentials found, using")
		}
	}

	return creds, nil
}
