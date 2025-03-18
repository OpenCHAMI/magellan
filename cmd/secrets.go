package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	secretsFile           string
	secretsStoreFormat    string
	secretsStoreInputFile string
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage credentials for BMC nodes",
	Long: "Manage credentials for BMC nodes to for querying information through redfish. This requires generating a key and setting the 'MASTER_KEY' environment variable for the secrets store.\n" +
		"Examples:\n\n" +
		"    export MASTER_KEY=$(magellan secrets generatekey)\n" +
		// store specific BMC node creds for `collect` and `crawl` in default secrets store (`--file/-f`` flag not set)
		"    magellan secrets store $bmc_host $bmc_creds" +
		// retrieve creds from secrets store
		"    magellan secrets retrieve $bmc_host -f nodes.json" +
		// list creds from specific secrets
		"    magellan secrets list -f nodes.json",
	Run: func(cmd *cobra.Command, args []string) {
		// show command help and exit
		if len(args) < 1 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

var secretsGenerateKeyCmd = &cobra.Command{
	Use:   "generatekey",
	Args:  cobra.NoArgs,
	Short: "Generates a new 32-byte master key (in hex).",
	Run: func(cmd *cobra.Command, args []string) {
		key, err := secrets.GenerateMasterKey()
		if err != nil {
			fmt.Printf("Error generating master key: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", key)
	},
}

var secretsStoreCmd = &cobra.Command{
	Use:   "store secretID <json(default)|base64>",
	Args:  cobra.MinimumNArgs(1),
	Short: "Stores the given string value under secretID.",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			secretID       string = args[0]
			secretValue    string
			store          secrets.SecretStore
			inputFileBytes []byte
			err            error
		)

		// require either the args or input file
		if len(args) < 1 && secretsStoreInputFile == "" {
			log.Error().Msg("no input data or file")
			os.Exit(1)
		} else if len(args) > 1 && secretsStoreInputFile == "" {
			secretValue = args[1]
		}

		switch secretsStoreFormat {
		case "base64":
			decoded, err := base64.StdEncoding.DecodeString(secretValue)
			if err != nil {
				fmt.Printf("Error decoding base64 data: %v\n", err)
				os.Exit(1)
			}

			// check the decoded string if it's a valid JSON and has creds
			if !isValidCredsJSON(string(decoded)) {
				log.Error().Msg("value is not a valid JSON or is missing credentials")
				os.Exit(1)
			}

			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			secretValue = string(decoded)
		case "json":
			// read input from file if set and override
			if secretsStoreInputFile != "" {
				if secretValue != "" {
					log.Error().Msg("cannot use -i/--input-file with positional argument")
					os.Exit(1)
				}
				inputFileBytes, err = os.ReadFile(secretsStoreInputFile)
				if err != nil {
					log.Error().Err(err).Msg("failed to read input file")
					os.Exit(1)
				}
				secretValue = string(inputFileBytes)
			}

			// make sure we have valid JSON with "username" and "password" properties
			if !isValidCredsJSON(string(secretValue)) {
				log.Error().Err(err).Msg("not a valid JSON or creds")
				os.Exit(1)
			}
			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		default:
			log.Error().Msg("no input format set")
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, secretValue); err != nil {
			fmt.Printf("Error storing secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Secret stored successfully.")
	},
}

func isValidCredsJSON(val string) bool {
	var (
		valid bool = !json.Valid([]byte(val))
		creds map[string]string
		err   error
	)
	err = json.Unmarshal([]byte(val), &creds)
	if err != nil {
		return false
	}
	_, valid = creds["username"]
	_, valid = creds["password"]
	return valid
}

var secretsRetrieveCmd = &cobra.Command{
	Use:  "retrieve secretID",
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var (
			secretID    = args[0]
			secretValue string
			store       secrets.SecretStore
			err         error
		)

		store, err = secrets.OpenStore(secretsFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		secretValue, err = store.GetSecretByID(secretID)
		if err != nil {
			fmt.Printf("Error retrieving secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Secret for %s: %s\n", secretID, secretValue)
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.MinimumNArgs(1),
	Short: "Lists all the secret IDs and their values.",
	Run: func(cmd *cobra.Command, args []string) {
		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		secrets, err := store.ListSecrets()
		if err != nil {
			fmt.Printf("Error listing secrets: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Secrets:")
		for key, value := range secrets {
			fmt.Printf("%s: %s\n", key, value)
		}
	},
}

func init() {
	secretsCmd.Flags().StringVarP(&secretsFile, "file", "f", "nodes.json", "")
	secretsStoreCmd.Flags().StringVar(&secretsStoreFormat, "format", "json", "set the input format for the secrets file (json|base64)")
	secretsStoreCmd.Flags().StringVarP(&secretsStoreInputFile, "input-file", "i", "", "set the file to read as input")

	secretsCmd.AddCommand(secretsGenerateKeyCmd)
	secretsCmd.AddCommand(secretsStoreCmd)
	secretsCmd.AddCommand(secretsRetrieveCmd)
	secretsCmd.AddCommand(secretsListCmd)

	rootCmd.AddCommand(secretsCmd)

	checkBindFlagError(viper.BindPFlags(secretsCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsStoreCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
}
