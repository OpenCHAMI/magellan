package util

import (
	"encoding/json"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Create a secret store by using credentials explicitly provided via Viper, or loading
// from the secrets file (and overriding that with explicit credentials, if any)
func BuildSecretStore() secrets.SecretStore {
	// Use secret store for BMC credentials, and/or credential CLI flags
	var store secrets.SecretStore
	if viper.IsSet("username") && viper.IsSet("password") {
		// First, try and load credentials from --username and --password if both are set.
		log.Debug().Msgf("--username and --password specified, using them for BMC credentials")
		store = secrets.NewStaticStore(viper.GetString("username"), viper.GetString("password"))
	} else {
		// Alternatively, locate specific credentials (falling back to default) and override those
		// with --username or --password if either are passed.
		secretsFile := viper.GetString("secrets.file")
		log.Debug().Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
		var err error
		if store, err = secrets.OpenStore(secretsFile); err != nil {
			log.Error().Err(err).Msg("failed to open local secrets store")
		}

		// Temporarily override username/password of each BMC if one of those
		// flags is passed. The expectation is that if the flag is specified
		// on the command line, it should be used.
		if viper.IsSet("username") {
			log.Info().Msg("--username passed, temporarily overriding all usernames from secret store with value")
		}
		if viper.IsSet("password") {
			log.Info().Msg("--password passed, temporarily overriding all passwords from secret store with value")
		}
		switch s := store.(type) {
		case *secrets.StaticStore:
			if viper.IsSet("username") {
				s.Username = viper.GetString("username")
			}
			if viper.IsSet("password") {
				s.Password = viper.GetString("password")
			}
		case *secrets.LocalSecretStore:
			for k := range s.Secrets {
				if creds, err := bmc.GetBMCCredentials(store, k); err != nil {
					log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials")
				} else {
					if viper.IsSet("username") {
						creds.Username = viper.GetString("username")
					}
					if viper.IsSet("password") {
						creds.Password = viper.GetString("password")
					}

					if newCreds, err := json.Marshal(creds); err != nil {
						log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials: marshal error")
					} else {
						s.StoreSecretByID(k, string(newCreds))
					}
				}
			}
		}
	}

	return store
}

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
