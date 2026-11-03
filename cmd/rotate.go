package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/spf13/cobra"
)

var rotationPasswordFile string

var rotateAccountCmd = &cobra.Command{
	Use:   "rotate-account <bmc> <account-id>",
	Short: "Change a BMC login password and save it to OpenBao (lab workflow)",
	Long: `Change the account matching the stored login username using a replacement
password from a private file. Requires --secrets-backend bao and an exact BMC URI
entry. Retain the file until verification is complete. There is no automatic
recovery or concurrent-writer protection. Use only a recoverable lab account.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if secretsBackend != secrets.BackendBao {
			return fmt.Errorf("this lab workflow requires --secrets-backend bao")
		}
		endpoint, err := bmc.SettingsEndpoint(args[0])
		if err != nil {
			return err
		}
		info, err := os.Stat(rotationPasswordFile)
		if err != nil {
			return fmt.Errorf("read replacement password file: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("replacement password must be a regular file accessible only by its owner (chmod 600)")
		}
		data, err := os.ReadFile(rotationPasswordFile)
		if err != nil {
			return err
		}
		newPassword := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		if newPassword == "" || strings.ContainsAny(newPassword, "\r\n") {
			return fmt.Errorf("password file must contain one nonempty line")
		}
		store, err := openSecretStore("")
		if err != nil {
			return err
		}
		if err := bmc.RotateStoredAccount(endpoint, args[1], newPassword, store, insecureScope.BMC(), cacertPath); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "BMC password verified and updated credential saved to OpenBao.")
		return err
	},
}

func init() {
	rotateAccountCmd.Flags().StringVar(&rotationPasswordFile, "new-password-file", "", "Private file containing the replacement password; retain for manual recovery")
	rotateAccountCmd.Flags().StringVar(&cacertPath, "cacert", "", "CA certificate bundle")
	registerInsecureFlag(rotateAccountCmd, "Skip TLS verification for the selected scope (lab only).")
	SettingsCmd.AddCommand(rotateAccountCmd)
}
