package cmd

import (
	"os"
	"strings"

	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	host             string
	firmwareUri      string
	firmwareVersion  string
	component        string
	transferProtocol string
	showStatus       bool
	Insecure         bool
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

		// use secret store for BMC credentials, and/or credential CLI flags
		var (
			store secrets.SecretStore
			uri   = args[0]
			err   error
		)
		if username != "" && password != "" {
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Str("id", uri).Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Str("id", uri).Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Str("id", uri).Err(err).Msg("failed to open local secrets store")
			}

			// Either none of the flags were passed or only one of them were; get
			// credentials from secrets store to fill in the gaps.
			bmcCreds, _ := bmc.GetBMCCredentials(store, uri)
			nodeCreds := secrets.StaticStore{
				Username: bmcCreds.Username,
				Password: bmcCreds.Password,
			}

			// If either of the flags were passed, override the fetched
			// credentials with them.
			if username != "" {
				log.Info().Str("id", uri).Msg("--username was set, overriding username for this BMC")
				nodeCreds.Username = username
			}
			if password != "" {
				log.Info().Str("id", uri).Msg("--password was set, overriding password for this BMC")
				nodeCreds.Password = password
			}

			store = &nodeCreds
		}

		// get status if flag is set and exit
		for _, arg := range args {
			if showStatus {
				err := magellan.GetUpdateStatus(&magellan.UpdateParams{
					FirmwareURI:      firmwareUri,
					TransferProtocol: transferProtocol,
					Insecure:         Insecure,
					CollectParams: magellan.CollectParams{
						URI:         arg,
						SecretStore: store,
						Timeout:     timeout,
					},
				})
				if err != nil {
					log.Error().Err(err).Msgf("failed to get update status")
				}
				return
			}

			// initiate a remote update
			err := magellan.UpdateFirmwareRemote(&magellan.UpdateParams{
				FirmwareURI:      firmwareUri,
				TransferProtocol: strings.ToUpper(transferProtocol),
				Insecure:         Insecure,
				CollectParams: magellan.CollectParams{
					URI:         arg,
					SecretStore: store,
					Timeout:     timeout,
				},
			})
			if err != nil {
				log.Error().Err(err).Msgf("failed to update firmware")
			}
		}
	},
}

func init() {
	updateCmd.Flags().StringVarP(&username, "username", "u", "", "Set the BMC user")
	updateCmd.Flags().StringVarP(&password, "password", "p", "", "Set the BMC password")
	updateCmd.Flags().StringVar(&transferProtocol, "scheme", "https", "Set the transfer protocol")
	updateCmd.Flags().StringVar(&firmwareUri, "firmware-uri", "", "Set the URI to retrieve the firmware")
	updateCmd.Flags().BoolVar(&showStatus, "status", false, "Get the status of the update")
	updateCmd.Flags().BoolVarP(&Insecure, "insecure", "i", false, "Allow insecure connections to the server")

	checkBindFlagError(viper.BindPFlag("update.scheme", updateCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("update.firmware-uri", updateCmd.Flags().Lookup("firmware-uri")))
	checkBindFlagError(viper.BindPFlag("update.status", updateCmd.Flags().Lookup("status")))
	checkBindFlagError(viper.BindPFlag("update.insecure", updateCmd.Flags().Lookup("insecure")))

	rootCmd.AddCommand(updateCmd)
}
