package cmd

import (
	"fmt"
	"os/user"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	urlx "github.com/OpenCHAMI/magellan/internal/url"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The `collect` command fetches data from a collection of BMC nodes.
// This command should be ran after the `scan` to find available hosts
// on a subnet.
var CollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect system information by interrogating BMC node",
	Long: "Send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache.\n" +
		"See the 'scan' command on how to perform a scan.\n\n" +
		"Examples:\n" +
		"  magellan collect --cache ./assets.db --output ./logs --timeout 30 --cacert cecert.pem\n" +
		"  magellan collect --host smd.example.com --port 27779 --username $username --password $password\n\n" +
		// example using `collect`
		"  export MASTER_KEY=$(magellan secrets generatekey)\n" +
		"  magellan secrets store $node_creds_json -f nodes.json" +
		"  magellan collect --host openchami.cluster --username $username --password $password \\\n",
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		scannedResults, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get scanned results from cache")
		}

		// URL sanitanization for host argument
		host, err = urlx.Sanitize(host)
		if err != nil {
			log.Error().Err(err).Msg("failed to sanitize host")
		}

		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = auth.LoadAccessToken(tokenPath)
			if err != nil && verbose {
				log.Warn().Err(err).Msgf("could not load access token")
			}
		}

		// set the minimum/maximum number of concurrent processes
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(scannedResults), 1, 10000)
		}

		// set the collect parameters from CLI params
		params := &magellan.CollectParams{
			URI:         host,
			Timeout:     timeout,
			Concurrency: concurrency,
			Verbose:     verbose,
			CaCertPath:  cacertPath,
			OutputPath:  outputPath,
			ForceUpdate: forceUpdate,
			AccessToken: accessToken,
			SecretsFile: secretsFile,
			Username:    username,
			Password:    password,
		}

		// show all of the 'collect' parameters being set from CLI if verbose
		if verbose {
			log.Debug().Any("params", params)
		}

		// load the secrets file to get node credentials by ID (i.e. the BMC node's URI)
		store, err := secrets.OpenStore(params.SecretsFile)
		if err != nil {
			// Something went wrong with the store so try using
			// Create a StaticSecretStore to hold the username and password
			log.Warn().Err(err).Msg("failed to open local store")
			store = secrets.NewStaticStore(username, password)
		}

		_, err = magellan.CollectInventory(&scannedResults, params, store)
		if err != nil {
			log.Error().Err(err).Msg("failed to collect data")
		}
	},
}

func init() {
	currentUser, _ = user.Current()
	CollectCmd.PersistentFlags().StringVar(&host, "host", "", "Set the URI to the SMD root endpoint")
	CollectCmd.PersistentFlags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	CollectCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	CollectCmd.PersistentFlags().StringVar(&secretsFile, "secrets-file", "", "Set path to the node secrets file")
	CollectCmd.PersistentFlags().StringVar(&scheme, "scheme", "https", "Set the default scheme used to query when not included in URI")
	CollectCmd.PersistentFlags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query")
	CollectCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", fmt.Sprintf("/tmp/%smagellan/inventory/", currentUser.Username+"/"), "Set the path to store collection data")
	CollectCmd.PersistentFlags().BoolVar(&forceUpdate, "force-update", false, "Set flag to force update data sent to SMD")
	CollectCmd.PersistentFlags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file. (defaults to system CAs when blank)")

	// set flags to only be used together
	CollectCmd.MarkFlagsRequiredTogether("username", "password")

	// bind flags to config properties
	checkBindFlagError(viper.BindPFlag("collect.host", CollectCmd.Flags().Lookup("host")))
	checkBindFlagError(viper.BindPFlag("collect.username", CollectCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("collect.password", CollectCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("collect.scheme", CollectCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("collect.protocol", CollectCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("collect.output", CollectCmd.Flags().Lookup("output")))
	checkBindFlagError(viper.BindPFlag("collect.force-update", CollectCmd.Flags().Lookup("force-update")))
	checkBindFlagError(viper.BindPFlag("collect.cacert", CollectCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(CollectCmd.Flags()))

	rootCmd.AddCommand(CollectCmd)
}
