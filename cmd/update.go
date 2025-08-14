package cmd

import (
	"os"
	"strings"

	"github.com/OpenCHAMI/magellan/internal/util"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The `update` command provides an interface to easily update firmware
// using Redfish. It also provides a simple way to check the status of
// an update in-progress.
var updateCmd = &cobra.Command{
	Use: "update hosts...",
	Example: `  // perform an firmware update
  magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password \
    --firmware-url http://172.16.0.200:8005/firmware/bios/image.RBU \
    --component BIOS

  // check update status
  magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password --status`,
	Short: "Update BMC node firmware",
	Long:  "Perform an firmware update using Redfish by providing a remote firmware URL and component.",
	Run: func(cmd *cobra.Command, args []string) {
		// check that we have at least one host
		if len(args) <= 0 {
			log.Error().Msg("update requires at least one host")
			os.Exit(1)
		}

		// Build secret store, using Viper parameters
		store := util.BuildSecretStore()

		// get status if flag is set and exit
		for _, arg := range args {
			firmwareUri := viper.GetString("update.firmware-uri")
			transferProtocol := viper.GetString("update.scheme")
			insecure := viper.GetBool("insecure")
			if viper.GetBool("update.status") {
				err := magellan.GetUpdateStatus(&magellan.UpdateParams{
					URI:              arg,
					FirmwareURI:      firmwareUri,
					TransferProtocol: transferProtocol,
					Insecure:         insecure,
					CollectParams: magellan.CollectParams{
						SecretStore: store,
						Timeout:     viper.GetInt("timeout"),
					},
				})
				if err != nil {
					log.Error().Err(err).Msgf("failed to get update status")
				}
				return
			}

			// initiate a remote update
			err := magellan.UpdateFirmwareRemote(&magellan.UpdateParams{
				URI:              arg,
				FirmwareURI:      firmwareUri,
				TransferProtocol: strings.ToUpper(transferProtocol),
				Insecure:         insecure,
				CollectParams: magellan.CollectParams{
					SecretStore: store,
					Timeout:     viper.GetInt("timeout"),
				},
			})
			if err != nil {
				log.Error().Err(err).Msgf("failed to update firmware")
			}
		}
	},
}

func init() {
	updateCmd.Flags().StringP("username", "u", "", "Set the BMC user")
	updateCmd.Flags().StringP("password", "p", "", "Set the BMC password")
	updateCmd.Flags().String("scheme", "https", "Set the transfer protocol")
	updateCmd.Flags().String("firmware-uri", "", "Set the URI to retrieve the firmware")
	updateCmd.Flags().Bool("status", false, "Get the status of the update")
	updateCmd.Flags().BoolP("insecure", "i", false, "Allow insecure connections to the server")

	checkBindFlagError(viper.BindPFlag("username", updateCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("password", updateCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("insecure", updateCmd.Flags().Lookup("insecure")))
	checkBindFlagError(viper.BindPFlag("update.scheme", updateCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("update.firmware-uri", updateCmd.Flags().Lookup("firmware-uri")))
	checkBindFlagError(viper.BindPFlag("update.status", updateCmd.Flags().Lookup("status")))

	rootCmd.AddCommand(updateCmd)
}
