package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	Use:   "store secretID secretValue",
	Args:  cobra.ExactArgs(2),
	Short: "Stores the given string value under secretID.",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			secretID    = args[0]
			secretValue = args[1]
		)

		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, secretValue); err != nil {
			fmt.Printf("Error storing secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Secret stored successfully.")
	},
}

var secretsStoreBase64Cmd = &cobra.Command{
	Use:   "storebase64 base64String",
	Args:  cobra.ExactArgs(1),
	Short: "Decodes the base64-encoded string before storing.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(os.Args) < 4 {
			fmt.Println("Not enough arguments. Usage: go run main.go storebase64 <secretID> <base64String> [filename]")
			os.Exit(1)
		}
		secretID := os.Args[2]
		base64Value := os.Args[3]
		filename := "mysecrets.json"
		if len(os.Args) == 5 {
			filename = os.Args[4]
		}

		decoded, err := base64.StdEncoding.DecodeString(base64Value)
		if err != nil {
			fmt.Printf("Error decoding base64 data: %v\n", err)
			os.Exit(1)
		}

		store, err := secrets.OpenStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, string(decoded)); err != nil {
			fmt.Printf("Error storing base64-decoded secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Base64-decoded secret stored successfully.")
	},
}

var secretsRetrieveCmd = &cobra.Command{
	Use: "retrieve secretID",
	Run: func(cmd *cobra.Command, args []string) {
		if len(os.Args) < 3 {
			fmt.Println("Not enough arguments. Usage: go run main.go retrieve <secretID> [filename]")
			os.Exit(1)
		}
		secretID := os.Args[2]

		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		secretValue, err := store.GetSecretByID(secretID)
		if err != nil {
			fmt.Printf("Error retrieving secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Secret for %s: %s\n", secretID, secretValue)
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all the secret IDs and their values.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			fmt.Println("Not enough arguments. Usage: go run main.go list [filename]")
			os.Exit(1)
		}

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

	secretsCmd.AddCommand(secretsGenerateKeyCmd)
	secretsCmd.AddCommand(secretsStoreCmd)
	secretsCmd.AddCommand(secretsStoreBase64Cmd)
	secretsCmd.AddCommand(secretsRetrieveCmd)
	secretsCmd.AddCommand(secretsListCmd)

	checkBindFlagError(viper.BindPFlags(secretsCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsStoreCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
	checkBindFlagError(viper.BindPFlags(secretsGenerateKeyCmd.Flags()))
}
