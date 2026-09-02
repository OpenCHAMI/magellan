package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	secretsFile           string
	secretsStoreInputFile string // this is for inputs not the store itself
	secretsStoreFormat    string // slightly different from format.DataFormat
	secretsListFormat     format.DataFormat
)

var secretsCmd = &cobra.Command{
	Use: "secrets",
	Example: `  # generate new key and set environment variable
  export MASTER_KEY=$(magellan secrets generatekey)

  # store specific BMC node creds for collect and crawl in default secrets store (--file/-f flag not set)
  magellan secrets store $bmc_host $username:$password

  # retrieve creds from secrets store
  magellan secrets retrieve $bmc_host -f secrets.json

  # list creds from specific secrets
  magellan secrets list -f nodes.json`,
	Short: "Manage credentials for BMC nodes",
	Long:  "Manage credentials for BMC nodes for querying information through Redfish. This requires generating a key and setting the 'MASTER_KEY' environment variable for the secrets store.",
}

var secretsGenerateKeyCmd = &cobra.Command{
	Use:   "generatekey",
	Args:  cobra.NoArgs,
	Short: "Generates a new 32-byte master key (in hex).",
	Run: func(cmd *cobra.Command, args []string) {
		key, err := secrets.GenerateMasterKey()
		if err != nil {
			log.Error().Err(err).Msg("failed to generate master key")
			return
		}
		fmt.Printf("%s\n", key)
	},
}

var secretsStoreCmd = &cobra.Command{
	Use: "store secretID <basic(default)|json|base64>",
	Example: `  # store a default username and password using basic format
  magellan secrets store default $username:$password

  # store credentials for specific host in JSON
  magellan secrets store $bmc_host '{"username": "$username", "password": "$password"}' 
	`,
	Short: "Stores the given string value under secretID.",
	Long: `Stores the given string value under secretID. The secretID string should
be in the format specified with '-F/--format'. If the '--format' is set to 'basic',
the secretID takes the form <username>:<password> similar to '-u' with 'curl'.`,
	Args: func(cmd *cobra.Command, args []string) error {

		if len(args) < 1 {
			return fmt.Errorf("expected at least one argument")
		} else if len(args) < 1 && secretsStoreInputFile == "" {
			log.Error().Msg("requires input data or file")
			return fmt.Errorf("must have input data or secrets (-f/--s)")
		}
		return nil
	}, //cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var (
			secretID       = args[0]
			secretValue    string
			store          secrets.SecretStore
			inputFileBytes []byte
			err            error
		)

		// require either the args or input file
		if len(args) < 1 && secretsStoreInputFile == "" {
			log.Error().Msg("requires input data or file")
			return
		} else if len(args) > 1 && secretsStoreInputFile == "" {
			// use args[1] here because args[0] is the secretID
			secretValue = args[1]
		}

		// handle input file format
		switch secretsStoreFormat {
		case "basic": // format: $username:$password
			var (
				values   []string
				username string
				password string
			)

			// seperate username and password provided
			values = strings.Split(secretValue, ":")
			if len(values) != 2 {
				log.Error().Msgf("expected 2 arguments in [username:password] format but got %d", len(values))
				return
			}

			// open secret store to save credentials
			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				log.Error().Err(err).Msg("failed to open secrets store")
				return
			}

			// extract username/password from input (for clarity)
			username = values[0]
			password = values[1]

			// create JSON formatted string from input
			secretValue = fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", username, password)

		case "base64": // format: ($encoded_base64_string)
			decoded, err := base64.StdEncoding.DecodeString(secretValue)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("failed to decode base64 data")
				return
			}

			// check the decoded string if it's a valid JSON and has creds
			if !isValidCredsJSON(string(decoded)) {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("invalid JSON value or is missing credentials")
				return
			}

			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("failed to open secrets store")
				os.Exit(1)
			}
			secretValue = string(decoded)
		case "json": // format: {"username": $username, "password": $password}
			// read input from file if set and override
			if secretsStoreInputFile != "" {
				if secretValue != "" {
					log.Error().
						Str("input-file", secretsStoreInputFile).
						Msg("cannot use -i/--input-file with positional argument")
					return
				}
				inputFileBytes, err = os.ReadFile(secretsStoreInputFile)
				if err != nil {
					log.Error().
						Err(err).
						Msg("failed to read input file")
					return
				}
				secretValue = string(inputFileBytes)
			}

			// make sure we have valid JSON with "username" and "password" properties
			if !isValidCredsJSON(secretValue) {
				log.Error().
					Err(err).
					Msg("invalid JSON value or creds")
				os.Exit(1)
			}
			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("failed to open secret store")
				os.Exit(1)
			}
		default:
			log.Error().Msg("invalid format (see --format flag for options)")
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, secretValue); err != nil {
			log.Error().
				Err(err).
				Str("id", secretID).
				Str("path", secretsFile).
				Msg("failed to store secret by ID")
			os.Exit(1)
		}
	},
}

func isValidCredsJSON(val string) bool {
	var (
		validUsername bool
		validPassword bool
		creds         map[string]string
		err           error
	)
	err = json.Unmarshal([]byte(val), &creds)
	if err != nil {
		return false
	}
	_, validUsername = creds["username"]
	_, validPassword = creds["password"]
	return json.Valid([]byte(val)) && validUsername && validPassword
}

var secretsRetrieveCmd = &cobra.Command{
	Use:   "retrieve [secretID]",
	Args:  cobra.MinimumNArgs(1),
	Short: "Retrieve the value of specific secret ID.",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			secretID    = args[0]
			secretValue string
			store       secrets.SecretStore
			err         error
		)

		store, err = secrets.OpenStore(secretsFile)
		if err != nil {
			log.Error().Err(err).Msg("failed to open secret store")
			return
		}

		secretValue, err = store.GetSecretByID(secretID)
		if err != nil {
			log.Error().Err(err).Msg("failed to retrieve secrets")
			return
		}
		fmt.Printf("Secret for %s: %s\n", secretID, secretValue)
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.ExactArgs(0),
	Short: "Lists all the secret IDs and their values.",
	Run: func(cmd *cobra.Command, args []string) {
		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			fmt.Println(err)
			return
		}

		secrets, err := store.ListSecrets()
		if err != nil {
			log.Error().Err(err).Msg("failed to list secrets")
			return
		}

		switch secretsListFormat {
		case format.FORMAT_JSON, format.FORMAT_YAML:
			format.MarshalData(secrets, secretsListFormat)
		case format.FORMAT_LIST:
			fallthrough
		default:
			for key, value := range secrets {
				fmt.Printf("%s: %s\n", key, value)
			}
		}
	},
}

var secretsRemoveCmd = &cobra.Command{
	Use:   "remove secret_ids...",
	Args:  cobra.MinimumNArgs(1),
	Short: "Remove secrets by IDs from secret store.",
	Run: func(cmd *cobra.Command, args []string) {
		for _, secretID := range args {
			// open secret store from file
			store, err := secrets.OpenStore(secretsFile)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("failed to open secret store")
				return
			}

			// remove secret from store by it's ID
			err = store.RemoveSecretByID(secretID)
			if err != nil {
				log.Error().
					Err(err).
					Str("id", secretID).
					Str("path", secretsFile).
					Msg("failed to remove secret")
				return
			}

			// update store by saving to original file
			err = secrets.SaveSecrets(secretsFile, store.(*secrets.LocalSecretStore).Secrets)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", secretsFile).
					Msg("failed to save secrets to file")
				return
			}
		}
	},
}

func init() {
	secretsCmd.PersistentFlags().StringVar(&secretsFile, "file", "secrets.json", "Set the secrets file with BMC credentials.")
	secretsStoreCmd.Flags().StringVarP(&secretsStoreFormat, "input-format", "f", "basic", "Set the input format for the secrets file (basic|json|base64).")
	secretsStoreCmd.Flags().StringVarP(&secretsStoreInputFile, "input-file", "i", "", "Set the file to read as input with credentials. The file must match the format specified with '--format'.")
	secretsListCmd.Flags().VarP(&secretsListFormat, "output-format", "F", "Set the output format to list secrets.")

	secretsCmd.AddCommand(secretsGenerateKeyCmd, secretsStoreCmd, secretsRetrieveCmd, secretsListCmd, secretsRemoveCmd)

	rootCmd.AddCommand(secretsCmd)

}
