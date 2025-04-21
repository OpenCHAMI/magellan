package util

import (
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
)

func GetBMCCredentials(store secrets.SecretStore, id string) bmc.BMCCredentials {
	var (
		creds bmc.BMCCredentials
		err   error
	)

	if id == "" {
		log.Error().Msg("failed to get BMC credentials: id was empty")
		return creds
	}

	if id == secrets.DEFAULT_KEY {
		log.Info().Msg("fetching default credentials")
		if creds, err = bmc.GetBMCCredentialsDefault(store); err != nil {
			log.Warn().Err(err).Msg("failed to get default credentials")
		} else {
			log.Info().Msg("default credentials found, using")
		}
		return creds
	}

	if creds, err = bmc.GetBMCCredentials(store, id); err != nil {
		// Specific credentials for URI not found, fetch default.
		log.Warn().Str("id", id).Msg("specific credentials not found, falling back to default")
		if defaultSecret, err := bmc.GetBMCCredentialsDefault(store); err != nil {
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
