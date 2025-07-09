package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	urlx "github.com/OpenCHAMI/magellan/internal/url"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var collectOutputFormat string

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
	PreRunE: func(cmd *cobra.Command, args []string) (error) {
		// Validate the specified file format
		if collectOutputFormat != "json" && collectOutputFormat != "yaml" {
			return fmt.Errorf("specified format '%s' is invalid, must be (json|yaml)", collectOutputFormat)
		}
		return nil
	},
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

		// use secret store for BMC credentials, and/or credential CLI flags
		var store secrets.SecretStore
		if username != "" && password != "" {
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Err(err).Msg("failed to open local secrets store")
			}

			// Temporarily override username/password of each BMC if one of those
			// flags is passed. The expectation is that if the flag is specified
			// on the command line, it should be used.
			if username != "" {
				log.Info().Msg("--username passed, temporarily overriding all usernames from secret store with value")
			}
			if password != "" {
				log.Info().Msg("--password passed, temporarily overriding all passwords from secret store with value")
			}
			switch s := store.(type) {
			case *secrets.StaticStore:
				if username != "" {
					s.Username = username
				}
				if password != "" {
					s.Password = password
				}
			case *secrets.LocalSecretStore:
				for k, _ := range s.Secrets {
					if creds, err := bmc.GetBMCCredentials(store, k); err != nil {
						log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials")
					} else {
						if username != "" {
							creds.Username = username
						}
						if password != "" {
							creds.Password = password
						}

						if newCreds, err := json.Marshal(creds); err != nil {
							log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials: marshal error")
						} else {
							s.StoreSecretByID(k, string(newCreds))
						}
					}
				}
			}
		}

		// set the collect parameters from CLI params
		params := &magellan.CollectParams{
			URI:         host,
			Timeout:     timeout,
			Concurrency: concurrency,
			Verbose:     verbose,
			CaCertPath:  cacertPath,
			OutputPath:  outputPath,
			OutputDir:   outputDir,
			Format:      collectOutputFormat,
			ForceUpdate: forceUpdate,
			AccessToken: accessToken,
			SecretStore: store,
			BMCIdMap:    idMapPath,
		}

		// show all of the 'collect' parameters being set from CLI if verbose
		if verbose {
			log.Debug().Any("params", params)
		}

		_, err = magellan.CollectInventory(&scannedResults, params)
		if err != nil {
			log.Error().Err(err).Msg("failed to collect data")
		}
	},
}

func init() {
	CollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	CollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	CollectCmd.Flags().StringVar(&secretsFile, "secrets-file", "", "Set path to the node secrets file")
	CollectCmd.Flags().StringVar(&scheme, "scheme", "https", "Set the default scheme used to query when not included in URI")
	CollectCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query")
	CollectCmd.Flags().StringVarP(&outputPath, "output-file", "o", "", "Set the path to store collection data using HIVE partitioning")
	CollectCmd.Flags().StringVarP(&outputDir, "output-dir", "O", "", "Set the path to store collection data using HIVE partitioning")
	CollectCmd.Flags().BoolVar(&forceUpdate, "force-update", false, "Set flag to force update data sent to SMD")
	CollectCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	CollectCmd.Flags().StringVarP(&collectOutputFormat, "format", "F", FORMAT_JSON, "Set the output format (json|yaml)")
	CollectCmd.Flags().StringVar(&idMapPath, "bmc-id-map", "m", "Set the BMC ID mapping from raw json data or use @<path> to specify a file path (json or yaml)")

	CollectCmd.MarkFlagsMutuallyExclusive("output-file", "output-dir")

	// bind flags to config properties
	checkBindFlagError(viper.BindPFlag("collect.scheme", CollectCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("collect.protocol", CollectCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("collect.output-file", CollectCmd.Flags().Lookup("output-file")))
	checkBindFlagError(viper.BindPFlag("collect.output-dir", CollectCmd.Flags().Lookup("output-dir")))
	checkBindFlagError(viper.BindPFlag("collect.force-update", CollectCmd.Flags().Lookup("force-update")))
	checkBindFlagError(viper.BindPFlag("collect.cacert", CollectCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(CollectCmd.Flags()))

	rootCmd.AddCommand(CollectCmd)
}
