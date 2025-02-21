package cmd

import (
	"os"
	"strings"

	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	host             string
	firmwareUrl      string
	firmwareVersion  string
	component        string
	transferProtocol string
	showStatus       bool
)

// The `update` command provides an interface to easily update firmware
// using Redfish. It also provides a simple way to check the status of
// an update in-progress.
var updateCmd = &cobra.Command{
	Use:   "update hosts...",
	Short: "Update BMC node firmware",
	Long: "Perform an firmware update using Redfish by providing a remote firmware URL and component.\n\n" +
		"Examples:\n" +
		"  magellan update 172.16.0.108:443 --username bmc_username --password bmc_password --firmware-url http://172.16.0.200:8005/firmware/bios/image.RBU --component BIOS\n" +
		"  magellan update 172.16.0.108:443 --status --username bmc_username --password bmc_password",
	Run: func(cmd *cobra.Command, args []string) {
		// check that we have at least one host
		if len(args) <= 0 {
			log.Error().Msg("update requires at least one host")
			os.Exit(1)
		}

		// get status if flag is set and exit
		for _, arg := range args {
			if showStatus {
				err := magellan.GetUpdateStatus(&magellan.UpdateParams{
					FirmwarePath:     firmwareUrl,
					FirmwareVersion:  firmwareVersion,
					Component:        component,
					TransferProtocol: transferProtocol,
					CollectParams: magellan.CollectParams{
						URI:      arg,
						Username: username,
						Password: password,
						Timeout:  timeout,
					},
				})
				if err != nil {
					log.Error().Err(err).Msgf("failed to get update status")
				}
				return
			}

			// initiate a remote update
			err := magellan.UpdateFirmwareRemote(&magellan.UpdateParams{
				FirmwarePath:     firmwareUrl,
				FirmwareVersion:  firmwareVersion,
				Component:        component,
				TransferProtocol: strings.ToUpper(transferProtocol),
				CollectParams: magellan.CollectParams{
					URI:      host,
					Username: username,
					Password: password,
					Timeout:  timeout,
				},
			})
			if err != nil {
				log.Error().Err(err).Msgf("failed to update firmware")
			}
		}
	},
}

func init() {
	updateCmd.Flags().StringVar(&username, "username", "", "Set the BMC user")
	updateCmd.Flags().StringVar(&password, "password", "", "Set the BMC password")
	updateCmd.Flags().StringVar(&transferProtocol, "scheme", "https", "Set the transfer protocol")
	updateCmd.Flags().StringVar(&firmwareUrl, "firmware-url", "", "Set the path to the firmware")
	updateCmd.Flags().StringVar(&firmwareVersion, "firmware-version", "", "Set the version of firmware to be installed")
	updateCmd.Flags().StringVar(&component, "component", "", "Set the component to upgrade (BMC|BIOS)")
	updateCmd.Flags().BoolVar(&showStatus, "status", false, "Get the status of the update")

	checkBindFlagError(viper.BindPFlag("update.username", updateCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("update.password", updateCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("update.scheme", updateCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("update.firmware-url", updateCmd.Flags().Lookup("firmware-url")))
	checkBindFlagError(viper.BindPFlag("update.firmware-version", updateCmd.Flags().Lookup("firmware-version")))
	checkBindFlagError(viper.BindPFlag("update.component", updateCmd.Flags().Lookup("component")))
	checkBindFlagError(viper.BindPFlag("update.status", updateCmd.Flags().Lookup("status")))

	rootCmd.AddCommand(updateCmd)
}
