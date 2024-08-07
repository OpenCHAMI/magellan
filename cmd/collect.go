package cmd

import (
	"fmt"
	"os/user"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	forceUpdate bool
)

// The `collect` command fetches data from a collection of BMC nodes.
// This command should be ran after the `scan` to find available hosts
// on a subnet.
var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect system information by interrogating BMC node",
	Long: "Send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache.\n" +
		"See the 'scan' command on how to perform a scan.\n\n" +
		"Examples:\n" +
		"  magellan collect --cache ./assets.db --output ./logs --timeout 30 --cacert cecert.pem\n" +
		"  magellan collect --host smd.example.com --port 27779 --username username --password password",
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		scannedResults, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get scanned results from cache")
		}

		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = util.LoadAccessToken(tokenPath)
			if err != nil {
				log.Error().Err(err).Msgf("failed to load access token")
			}
		}

		if verbose {
			log.Debug().Str("Access Token", accessToken)
		}

		//
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(scannedResults), 1, 255)
		}
		err = magellan.CollectInventory(&scannedResults, &magellan.CollectParams{
			Username:    username,
			Password:    password,
			Timeout:     timeout,
			Concurrency: concurrency,
			Verbose:     verbose,
			CaCertPath:  cacertPath,
			OutputPath:  outputPath,
			ForceUpdate: forceUpdate,
			AccessToken: accessToken,
		})
		if err != nil {
			log.Error().Err(err).Msgf("failed to collect data")
		}
	},
}

func init() {
	currentUser, _ = user.Current()
	collectCmd.PersistentFlags().StringVar(&client.Host, "host", client.Host, "set the host to the SMD API")
	collectCmd.PersistentFlags().IntVarP(&client.Port, "port", "p", client.Port, "set the port to the SMD API")
	collectCmd.PersistentFlags().StringVar(&username, "username", "", "set the BMC user")
	collectCmd.PersistentFlags().StringVar(&password, "password", "", "set the BMC password")
	collectCmd.PersistentFlags().StringVar(&scheme, "scheme", "https", "set the scheme used to query")
	collectCmd.PersistentFlags().StringVar(&protocol, "protocol", "tcp", "set the protocol used to query")
	collectCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", fmt.Sprintf("/tmp/%smagellan/inventory/", currentUser.Username+"/"), "set the path to store collection data")
	collectCmd.PersistentFlags().BoolVar(&forceUpdate, "force-update", false, "set flag to force update data sent to SMD")
	collectCmd.PersistentFlags().StringVar(&cacertPath, "cacert", "", "path to CA cert. (defaults to system CAs)")

	// set flags to only be used together
	collectCmd.MarkFlagsRequiredTogether("username", "password")

	// bind flags to config properties
	viper.BindPFlag("collect.driver", collectCmd.Flags().Lookup("driver"))
	viper.BindPFlag("collect.host", collectCmd.Flags().Lookup("host"))
	viper.BindPFlag("collect.port", collectCmd.Flags().Lookup("port"))
	viper.BindPFlag("collect.username", collectCmd.Flags().Lookup("username"))
	viper.BindPFlag("collect.password", collectCmd.Flags().Lookup("password"))
	viper.BindPFlag("collect.protocol", collectCmd.Flags().Lookup("protocol"))
	viper.BindPFlag("collect.output", collectCmd.Flags().Lookup("output"))
	viper.BindPFlag("collect.force-update", collectCmd.Flags().Lookup("force-update"))
	viper.BindPFlag("collect.cacert", collectCmd.Flags().Lookup("secure-tls"))
	viper.BindPFlags(collectCmd.Flags())

	rootCmd.AddCommand(collectCmd)
}
