package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/spf13/cobra"
)

var (
	rotationPasswordSecretID string
	rotationSecretsFile      string
)

const rotationPasswordEnv = "MAGELLAN_NEW_PASSWORD"

var rotateAccountCmd = &cobra.Command{
	Use:   "rotate-account <bmc> <account-id>",
	Short: "Change a BMC login password using an environment variable or local encrypted secret",
	Long: `Change the account matching the stored login username using a replacement
password from MAGELLAN_NEW_PASSWORD, or from Magellan's local encrypted secrets
file when that environment variable is unset. The updated BMC credential is
saved to the selected --secrets-backend. There is no automatic recovery or
concurrent-writer protection. Use only a recoverable lab account.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := bmc.SettingsEndpoint(args[0])
		if err != nil {
			return err
		}
		newPassword, err := resolveRotationPassword(rotationPasswordSecretID, rotationSecretsFile)
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
	rotateAccountCmd.Flags().StringVar(&rotationPasswordSecretID, "new-password-secret-id", "", "Fallback ID of replacement credential in local encrypted store when MAGELLAN_NEW_PASSWORD is unset")
	rotateAccountCmd.Flags().StringVar(&rotationSecretsFile, "new-password-secrets-file", "secrets.json", "Local encrypted secrets file for replacement credential fallback")
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

func resolveRotationPassword(secretID, filename string) (string, error) {
	if value := os.Getenv(rotationPasswordEnv); value != "" {
		return value, nil
	}
	if secretID == "" {
		return "", fmt.Errorf("set %s or provide --new-password-secret-id for the local encrypted store", rotationPasswordEnv)
	}
	return readLocalRotationPassword(filename, secretID)
}
