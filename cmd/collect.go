package cmd

import (
	"encoding/json"
	"fmt"
	"os/user"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	urlx "github.com/OpenCHAMI/magellan/internal/url"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
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
	Use: "collect",
	Example: `  // basic collect after scan without making a follow-up request
  magellan collect --cache ./assets.db --cacert ochami.pem -o ./logs -t 30

  // set username and password for all nodes and make request to specified host
  magellan collect --host https://smd.openchami.cluster -u $bmc_username -p $bmc_password

  // run a collect using secrets manager with fallback username and password
  export MASTER_KEY=$(magellan secrets generatekey)
  magellan secrets store $node_creds_json -f nodes.json
  magellan collect --host https://smd.openchami.cluster -u $fallback_bmc_username -p $fallback_bmc_password`,
	Short: "Collect system information by interrogating BMC node",
	Long:  "Send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache.\nSee the 'scan' command on how to perform a scan.",
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
			log.Warn().Err(err).Msg("failed to open local store...falling back to default provided arguments")
			// try and use the `username` and `password` arguments instead
			store = secrets.NewStaticStore(username, password)
		}

		// found the store so try to load the creds
		_, err = store.GetSecretByID(host)
		if err != nil {
			// if we have CLI flags set, then we want to override default stored creds
			if username != "" && password != "" {
				// finally, use the CLI arguments passed instead
				log.Info().Msg("...using provided arguments for credentials")
				store = secrets.NewStaticStore(username, password)
			} else {
				// try and get a default *stored* username/password
				secret, err := store.GetSecretByID(secrets.DEFAULT_KEY)
				if err != nil {
					// no default found, so use CLI arguments
					log.Warn().Err(err).Msg("failed to get default credentials...")
				} else {
					// found default values in local store so use them
					log.Info().Msg("...using default store for credentials")
					var creds crawler.BMCUsernamePassword
					err = json.Unmarshal([]byte(secret), &creds)
					if err != nil {
						log.Warn().Err(err).Msg("failed to unmarshal default store credentials")
					}
				}
			}
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
	checkBindFlagError(viper.BindPFlag("collect.scheme", CollectCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("collect.protocol", CollectCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("collect.output", CollectCmd.Flags().Lookup("output")))
	checkBindFlagError(viper.BindPFlag("collect.force-update", CollectCmd.Flags().Lookup("force-update")))
	checkBindFlagError(viper.BindPFlag("collect.cacert", CollectCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(CollectCmd.Flags()))

	rootCmd.AddCommand(CollectCmd)
}
