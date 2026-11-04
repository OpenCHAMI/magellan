package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/spf13/cobra"
)

var (
	rotationPasswordSecretID string
	rotationSecretsFile      string
)

var rotateAccountCmd = &cobra.Command{
	Use:   "rotate-account <bmc> <account-id>",
	Short: "Change a BMC login password using a local encrypted secret",
	Long: `Change the account matching the stored login username using a replacement
credential read from Magellan's local encrypted secrets file. The updated BMC
credential is saved to the selected --secrets-backend. There is no automatic
recovery or concurrent-writer protection. Use only a recoverable lab account.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if rotationPasswordSecretID == "" {
			return fmt.Errorf("--new-password-secret-id is required")
		}
		endpoint, err := bmc.SettingsEndpoint(args[0])
		if err != nil {
			return err
		}
		newPassword, err := readLocalRotationPassword(rotationSecretsFile, rotationPasswordSecretID)
		if err != nil {
			return err
		}
		store, err := openSecretStore(bmc.SettingsSecretsFile)
		if err != nil {
			return err
		}
		if err := bmc.RotateStoredAccount(endpoint, args[1], newPassword, store, insecureScope.BMC(), cacertPath); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "BMC password verified and updated credential saved to the selected secrets backend.")
		return err
	},
}

func init() {
	rotateAccountCmd.Flags().StringVar(&rotationPasswordSecretID, "new-password-secret-id", "", "ID of the replacement credential in the local encrypted secrets file")
	rotateAccountCmd.Flags().StringVar(&rotationSecretsFile, "new-password-secrets-file", "secrets.json", "Local encrypted secrets file containing the replacement credential")
	rotateAccountCmd.Flags().StringVar(&bmc.SettingsSecretsFile, "secrets-file", "secrets.json", "Local secrets file for the selected destination backend")
	rotateAccountCmd.Flags().StringVar(&cacertPath, "cacert", "", "CA certificate bundle")
	registerInsecureFlag(rotateAccountCmd, "Skip TLS verification for the selected scope (lab only).")
	SettingsCmd.AddCommand(rotateAccountCmd)
}

func readLocalRotationPassword(filename, secretID string) (string, error) {
	if filename == "" || secretID == "" {
		return "", fmt.Errorf("local secrets file and replacement secret ID are required")
	}
	store, err := secrets.OpenStore(filename)
	if err != nil {
		return "", fmt.Errorf("open local encrypted secrets file: %w", err)
	}
	value, err := store.GetSecretByID(secretID)
	if err != nil {
		return "", fmt.Errorf("read replacement credential %q from local encrypted secrets: %w", secretID, err)
	}
	var credential bmc.BMCCredentials
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return "", fmt.Errorf("replacement credential %q is not valid credential JSON: %w", secretID, err)
	}
	if credential.Password == "" {
		return "", fmt.Errorf("replacement credential %q has an empty password", secretID)
	}
	return credential.Password, nil
}
