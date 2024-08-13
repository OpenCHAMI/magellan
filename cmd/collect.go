package cmd

import (
	"fmt"
	"os/user"
	"strings"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		// URL sanitanization for host argument
		host = strings.TrimSuffix(host, "/")
		host = strings.ReplaceAll(host, "//", "/")

		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = auth.LoadAccessToken(tokenPath)
			if err != nil && verbose {
				log.Warn().Err(err).Msgf("could not load access token")
			}
		}

		if verbose {
			log.Debug().Str("Access Token", accessToken)
		}

		//
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(scannedResults), 1, 10000)
		}
		err = magellan.CollectInventory(&scannedResults, &magellan.CollectParams{
			URI:         host,
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
	collectCmd.PersistentFlags().StringVar(&host, "host", "", "Set the URI to the SMD root endpoint")
	collectCmd.PersistentFlags().StringVar(&username, "username", "", "Set the BMC user")
	collectCmd.PersistentFlags().StringVar(&password, "password", "", "Set the BMC password")
	collectCmd.PersistentFlags().StringVar(&scheme, "scheme", "https", "Set the scheme used to query")
	collectCmd.PersistentFlags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query")
	collectCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", fmt.Sprintf("/tmp/%smagellan/inventory/", currentUser.Username+"/"), "Set the path to store collection data")
	collectCmd.PersistentFlags().BoolVar(&forceUpdate, "force-update", false, "Set flag to force update data sent to SMD")
	collectCmd.PersistentFlags().StringVar(&cacertPath, "cacert", "", "Path to CA cert. (defaults to system CAs)")

	// set flags to only be used together
	collectCmd.MarkFlagsRequiredTogether("username", "password")

	// bind flags to config properties
	checkBindFlagError(viper.BindPFlag("collect.host", collectCmd.Flags().Lookup("host")))
	checkBindFlagError(viper.BindPFlag("collect.username", collectCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("collect.password", collectCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("collect.scheme", collectCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("collect.protocol", collectCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("collect.output", collectCmd.Flags().Lookup("output")))
	checkBindFlagError(viper.BindPFlag("collect.force-update", collectCmd.Flags().Lookup("force-update")))
	checkBindFlagError(viper.BindPFlag("collect.cacert", collectCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(collectCmd.Flags()))

	rootCmd.AddCommand(collectCmd)
}
